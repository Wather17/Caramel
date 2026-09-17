package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	wordXMLNamespace = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	documentXMLPath  = "word/document.xml"
)

type docxSplitBlock struct {
	node         *packageXMLNode
	sourceIndex  int
	endsASection bool
}

// SplitDOCX creates one DOCX per segment separated by explicit page or
// section breaks. It does not infer rendered page boundaries.
func SplitDOCX(inputPath, outputDir string) ([]string, error) {
	inputAbs, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, fmt.Errorf("caminho de entrada inválido '%s': %w", inputPath, err)
	}
	inputAbs = filepath.Clean(inputAbs)
	info, err := os.Stat(inputAbs)
	if err != nil {
		return nil, fmt.Errorf("não foi possível acessar o DOCX '%s': %w", inputPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("a entrada '%s' não é um arquivo regular", inputPath)
	}
	if !strings.EqualFold(filepath.Ext(inputAbs), ".docx") {
		return nil, fmt.Errorf("a entrada '%s' precisa ter extensão .docx", inputPath)
	}

	archive, err := zip.OpenReader(inputAbs)
	if err != nil {
		return nil, fmt.Errorf("não foi possível abrir '%s' como DOCX válido: %w", inputPath, err)
	}
	defer archive.Close()

	documentBytes, err := readMainDocumentXML(archive)
	if err != nil {
		return nil, err
	}
	xmlDocument, err := parsePackageXML(documentBytes)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler o XML principal de '%s': %w", inputPath, err)
	}
	if xmlDocument.root.name.Space != wordXMLNamespace || xmlDocument.root.name.Local != "document" {
		return nil, fmt.Errorf("'%s' não contém um documento WordprocessingML válido", inputPath)
	}
	body := xmlDocument.root.firstElement(wordXMLNamespace, "body")
	if body == nil {
		return nil, fmt.Errorf("o documento '%s' não contém w:body", inputPath)
	}

	bodyElements := body.directElements()
	var finalSection *packageXMLNode
	if len(bodyElements) > 0 && isWordElement(bodyElements[len(bodyElements)-1], "sectPr") {
		finalSection = bodyElements[len(bodyElements)-1]
		bodyElements = bodyElements[:len(bodyElements)-1]
	}
	for _, element := range bodyElements {
		if !isWordElement(element, "p") && containsNestedExplicitSplitMarker(element) {
			return nil, fmt.Errorf("quebras explícitas dentro de blocos OOXML aninhados não são suportadas; mova a quebra para um parágrafo de nível superior")
		}
	}

	parts := splitBodyAtExplicitBreaks(bodyElements)
	if len(parts) < 2 {
		return nil, fmt.Errorf("o DOCX não contém quebras explícitas com conteúdo nos dois lados; nenhuma saída foi criada")
	}

	base := strings.TrimSuffix(filepath.Base(inputAbs), filepath.Ext(inputAbs))
	if strings.TrimSpace(outputDir) == "" {
		outputDir = filepath.Join(filepath.Dir(inputAbs), base+"_split")
	}
	outputAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("caminho da pasta de saída inválido '%s': %w", outputDir, err)
	}
	outputAbs = filepath.Clean(outputAbs)
	if outputInfo, statErr := os.Stat(outputAbs); statErr == nil && !outputInfo.IsDir() {
		return nil, fmt.Errorf("o caminho de saída '%s' existe e não é uma pasta", outputDir)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("não foi possível acessar a pasta de saída '%s': %w", outputDir, statErr)
	}

	outputPaths := make([]string, len(parts))
	for i := range parts {
		outputPath := filepath.Join(outputAbs, fmt.Sprintf("%s_part_%03d.docx", base, i+1))
		if sameDOCXPath(inputAbs, outputPath) {
			return nil, fmt.Errorf("a saída '%s' não pode substituir o DOCX de entrada", outputPath)
		}
		if _, statErr := os.Lstat(outputPath); statErr == nil {
			return nil, fmt.Errorf("a saída '%s' já existe", outputPath)
		} else if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("não foi possível acessar a saída '%s': %w", outputPath, statErr)
		}
		outputPaths[i] = outputPath
	}

	if err := os.MkdirAll(outputAbs, 0o755); err != nil {
		return nil, fmt.Errorf("não foi possível criar a pasta de saída '%s': %w", outputDir, err)
	}
	stageDir, err := os.MkdirTemp(outputAbs, ".caramel-docx-split-")
	if err != nil {
		return nil, fmt.Errorf("não foi possível preparar as saídas DOCX: %w", err)
	}
	defer os.RemoveAll(stageDir)

	stagedPaths := make([]string, len(parts))
	for i, part := range parts {
		partDocument := &packageXMLDocument{
			prefix: append([]byte(nil), xmlDocument.prefix...),
			root:   xmlDocument.root.clone(),
			suffix: append([]byte(nil), xmlDocument.suffix...),
		}
		partBody := partDocument.root.firstElement(wordXMLNamespace, "body")
		partElements := make([]*packageXMLNode, 0, len(part.blocks)+1)
		for _, block := range part.blocks {
			partElements = append(partElements, block.node.clone())
		}
		if section := sectionPropertiesForPart(part, bodyElements, finalSection); section != nil {
			partElements = append(partElements, section.clone())
		}
		partBody.replaceElements(partElements)

		stagedPath := filepath.Join(stageDir, filepath.Base(outputPaths[i]))
		if err := writeDOCXWithDocument(archive, stagedPath, partDocument.bytes()); err != nil {
			return nil, fmt.Errorf("não foi possível preparar a parte %d: %w", i+1, err)
		}
		stagedPaths[i] = stagedPath
	}

	if err := publishDOCXFiles(stagedPaths, outputPaths); err != nil {
		return nil, err
	}
	return outputPaths, nil
}

type docxSplitPart struct {
	blocks []docxSplitBlock
}

func splitBodyAtExplicitBreaks(bodyElements []*packageXMLNode) []docxSplitPart {
	var parts []docxSplitPart
	var current []docxSplitBlock
	flush := func() {
		if splitBlocksHaveContent(current) {
			parts = append(parts, docxSplitPart{blocks: current})
		}
		current = nil
	}

	for index, element := range bodyElements {
		if !isWordElement(element, "p") {
			current = append(current, docxSplitBlock{node: element, sourceIndex: index})
			continue
		}

		if paragraphHasPageBreakBefore(element) {
			flush()
		}

		paragraphs, _ := splitParagraphAtManualPageBreaks(element)
		for partIndex, paragraph := range paragraphs {
			if partIndex > 0 {
				flush()
			}
			sectionEnd := paragraphSectionProperties(paragraph) != nil
			if packageXMLNodeHasContent(paragraph) || sectionEnd {
				current = append(current, docxSplitBlock{
					node:         paragraph,
					sourceIndex:  index,
					endsASection: sectionEnd,
				})
			}
		}

		if paragraphSectionProperties(element) != nil {
			flush()
		}
	}
	flush()
	return parts
}

func splitParagraphAtManualPageBreaks(paragraph *packageXMLNode) ([]*packageXMLNode, bool) {
	if !containsManualPageBreak(paragraph) {
		return []*packageXMLNode{paragraph.clone()}, false
	}
	parts := splitXMLNodeAtManualPageBreaks(paragraph)
	if len(parts) < 2 {
		return []*packageXMLNode{paragraph.clone()}, false
	}
	if paragraphSectionProperties(paragraph) != nil {
		for i := 0; i < len(parts)-1; i++ {
			removeParagraphSectionProperties(parts[i])
		}
	}
	return parts, true
}

func splitXMLNodeAtManualPageBreaks(node *packageXMLNode) []*packageXMLNode {
	parts := []*packageXMLNode{node.cloneShallow()}
	var repeatableProperties []*packageXMLNode
	for _, childPart := range node.parts {
		if childPart.node == nil {
			parts[len(parts)-1].parts = append(parts[len(parts)-1].parts, packageXMLPart{raw: childPart.raw})
			continue
		}
		child := childPart.node
		if isManualPageBreak(child) {
			parts = append(parts, node.cloneShallow())
			appendRepeatedProperties(parts[len(parts)-1], repeatableProperties)
			continue
		}

		childParts := splitXMLNodeAtManualPageBreaks(child)
		for i, childFragment := range childParts {
			if i > 0 {
				parts = append(parts, node.cloneShallow())
				appendRepeatedProperties(parts[len(parts)-1], repeatableProperties)
			}
			parts[len(parts)-1].parts = append(parts[len(parts)-1].parts, packageXMLPart{node: childFragment})
		}
		if repeatableWordProperty(node, child) {
			repeatableProperties = append(repeatableProperties, child)
		}
	}
	return parts
}

func (n *packageXMLNode) cloneShallow() *packageXMLNode {
	return &packageXMLNode{
		name:          n.name,
		qualifiedName: n.qualifiedName,
		attrs:         append([]xml.Attr(nil), n.attrs...),
		attrNames:     append([]string(nil), n.attrNames...),
		open:          append([]byte(nil), n.open...),
		close:         append([]byte(nil), n.close...),
		emptyTag:      n.emptyTag,
	}
}

func appendRepeatedProperties(target *packageXMLNode, properties []*packageXMLNode) {
	for _, property := range properties {
		target.parts = append(target.parts, packageXMLPart{node: property.clone()})
	}
}

func repeatableWordProperty(parent, child *packageXMLNode) bool {
	if parent.name.Space != wordXMLNamespace || child.name.Space != wordXMLNamespace {
		return false
	}
	return (parent.name.Local == "p" && child.name.Local == "pPr") ||
		(parent.name.Local == "r" && child.name.Local == "rPr")
}

func removeParagraphSectionProperties(paragraph *packageXMLNode) {
	pPr := paragraph.firstElement(wordXMLNamespace, "pPr")
	if pPr == nil {
		return
	}
	parts := pPr.parts[:0]
	for _, part := range pPr.parts {
		if part.node != nil && isWordElement(part.node, "sectPr") {
			continue
		}
		parts = append(parts, part)
	}
	pPr.parts = parts
}

func paragraphSectionProperties(paragraph *packageXMLNode) *packageXMLNode {
	pPr := paragraph.firstElement(wordXMLNamespace, "pPr")
	if pPr == nil {
		return nil
	}
	return pPr.firstElement(wordXMLNamespace, "sectPr")
}

func paragraphHasPageBreakBefore(paragraph *packageXMLNode) bool {
	pPr := paragraph.firstElement(wordXMLNamespace, "pPr")
	if pPr == nil {
		return false
	}
	property := pPr.firstElement(wordXMLNamespace, "pageBreakBefore")
	if property == nil {
		return false
	}
	for _, attr := range property.attrs {
		if attr.Name.Space != wordXMLNamespace || attr.Name.Local != "val" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(attr.Value)) {
		case "0", "false", "off":
			return false
		}
	}
	return true
}

func containsNestedExplicitSplitMarker(node *packageXMLNode) bool {
	if isWordElement(node, "p") && (paragraphHasPageBreakBefore(node) || paragraphSectionProperties(node) != nil || containsManualPageBreak(node)) {
		return true
	}
	if isManualPageBreak(node) {
		return true
	}
	for _, part := range node.parts {
		if part.node != nil && containsNestedExplicitSplitMarker(part.node) {
			return true
		}
	}
	return false
}

func containsManualPageBreak(node *packageXMLNode) bool {
	if isManualPageBreak(node) {
		return true
	}
	for _, part := range node.parts {
		if part.node != nil && containsManualPageBreak(part.node) {
			return true
		}
	}
	return false
}

func isManualPageBreak(node *packageXMLNode) bool {
	if !isWordElement(node, "br") {
		return false
	}
	for _, attr := range node.attrs {
		if attr.Name.Space == wordXMLNamespace && attr.Name.Local == "type" && attr.Value == "page" {
			return true
		}
	}
	return false
}

func isWordElement(node *packageXMLNode, local string) bool {
	return node != nil && node.name.Space == wordXMLNamespace && node.name.Local == local
}

func packageXMLNodeHasContent(node *packageXMLNode) bool {
	if node == nil {
		return false
	}
	if node.name.Space == wordXMLNamespace {
		switch node.name.Local {
		case "t", "delText", "instrText":
			return strings.TrimSpace(html.UnescapeString(rawXMLNodeText(node))) != ""
		case "tbl", "drawing", "pict", "object", "altChunk", "tab", "sym", "noBreakHyphen", "softHyphen":
			return true
		case "br":
			return !isManualPageBreak(node)
		}
	}
	for _, part := range node.parts {
		if part.node != nil && packageXMLNodeHasContent(part.node) {
			return true
		}
	}
	return false
}

func rawXMLNodeText(node *packageXMLNode) string {
	var buffer bytes.Buffer
	for _, part := range node.parts {
		if part.node != nil {
			part.node.appendTo(&buffer)
		} else {
			buffer.Write(part.raw)
		}
	}
	return buffer.String()
}

func splitBlocksHaveContent(blocks []docxSplitBlock) bool {
	for _, block := range blocks {
		if packageXMLNodeHasContent(block.node) {
			return true
		}
	}
	return false
}

func sectionPropertiesForPart(part docxSplitPart, sourceBody []*packageXMLNode, finalSection *packageXMLNode) *packageXMLNode {
	if len(part.blocks) == 0 {
		return finalSection
	}
	last := part.blocks[len(part.blocks)-1]
	if last.endsASection {
		return nil
	}
	for index := last.sourceIndex; index < len(sourceBody); index++ {
		if section := paragraphSectionProperties(sourceBody[index]); section != nil {
			return section
		}
	}
	return finalSection
}

func readMainDocumentXML(archive *zip.ReadCloser) ([]byte, error) {
	var documentFile *zip.File
	for _, file := range archive.File {
		if file.Name != documentXMLPath {
			continue
		}
		if documentFile != nil {
			return nil, fmt.Errorf("o DOCX contém mais de uma entrada '%s'", documentXMLPath)
		}
		documentFile = file
	}
	if documentFile == nil {
		return nil, fmt.Errorf("o DOCX não contém '%s'", documentXMLPath)
	}
	reader, err := documentFile.Open()
	if err != nil {
		return nil, fmt.Errorf("não foi possível abrir '%s': %w", documentXMLPath, err)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler '%s': %w", documentXMLPath, err)
	}
	return content, nil
}

func writeDOCXWithDocument(archive *zip.ReadCloser, path string, documentXML []byte) (retErr error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	for _, source := range archive.File {
		header := &zip.FileHeader{
			Name:     source.Name,
			Method:   zip.Deflate,
			Modified: source.Modified,
		}
		if source.FileInfo().IsDir() {
			header.Method = zip.Store
		}
		header.SetMode(source.Mode())
		entryWriter, createErr := writer.CreateHeader(header)
		if createErr != nil {
			retErr = fmt.Errorf("não foi possível criar a entrada '%s': %w", source.Name, createErr)
			break
		}
		if source.Name == documentXMLPath {
			if _, writeErr := entryWriter.Write(documentXML); writeErr != nil {
				retErr = fmt.Errorf("não foi possível escrever '%s': %w", documentXMLPath, writeErr)
				break
			}
			continue
		}
		reader, openErr := source.Open()
		if openErr != nil {
			retErr = fmt.Errorf("não foi possível abrir a entrada '%s': %w", source.Name, openErr)
			break
		}
		_, copyErr := io.Copy(entryWriter, reader)
		closeErr := reader.Close()
		if copyErr != nil || closeErr != nil {
			retErr = fmt.Errorf("não foi possível copiar a entrada '%s': %w", source.Name, errors.Join(copyErr, closeErr))
			break
		}
	}
	if closeErr := writer.Close(); closeErr != nil {
		retErr = errors.Join(retErr, closeErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		retErr = errors.Join(retErr, closeErr)
	}
	if retErr != nil {
		_ = os.Remove(path)
	}
	return retErr
}

func sameDOCXPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs) {
		return true
	}
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func removeDOCXOutputs(paths []string) error {
	var errs []error
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func publishDOCXFiles(stagedPaths, outputPaths []string) error {
	if len(stagedPaths) != len(outputPaths) {
		return fmt.Errorf("quantidade de arquivos preparados não corresponde às saídas")
	}
	committed := make([]string, 0, len(outputPaths))
	for i, outputPath := range outputPaths {
		staged, err := os.Open(stagedPaths[i])
		if err != nil {
			rollbackErr := removeDOCXOutputs(committed)
			return fmt.Errorf("não foi possível abrir a parte preparada '%s': %w", stagedPaths[i], errors.Join(err, rollbackErr))
		}
		output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			_ = staged.Close()
			rollbackErr := removeDOCXOutputs(committed)
			return fmt.Errorf("não foi possível publicar '%s': %w", outputPath, errors.Join(err, rollbackErr))
		}
		_, copyErr := io.Copy(output, staged)
		outputCloseErr := output.Close()
		stagedCloseErr := staged.Close()
		if err := errors.Join(copyErr, outputCloseErr, stagedCloseErr); err != nil {
			_ = os.Remove(outputPath)
			rollbackErr := removeDOCXOutputs(committed)
			return fmt.Errorf("não foi possível publicar '%s': %w", outputPath, errors.Join(err, rollbackErr))
		}
		committed = append(committed, outputPath)
	}
	return nil
}
