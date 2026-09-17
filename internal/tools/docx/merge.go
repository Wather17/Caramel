package docx

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	mergeDocumentPath     = "word/document.xml"
	mergeContentTypesPath = "[Content_Types].xml"
)

type mergeDOCXPackage struct {
	archive       *zip.ReadCloser
	files         map[string]*zip.File
	contentTypes  mergeContentTypes
	document      *packageXMLDocument
	documentRels  []opcRelationship
	stylesPath    string
	styles        *packageXMLDocument
	numberingPath string
	numbering     *packageXMLDocument
}

// MergeDOCX combines two or more DOCX packages in argument order. Each source
// begins a new page section in the merged document.
func MergeDOCX(inputPaths []string, outputPath string) error {
	if len(inputPaths) < 2 {
		return fmt.Errorf("informe ao menos dois arquivos DOCX de entrada")
	}
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("informe o caminho do arquivo DOCX de saída")
	}

	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("caminho de saída inválido '%s': %w", outputPath, err)
	}
	outputAbs = filepath.Clean(outputAbs)
	if !strings.EqualFold(filepath.Ext(outputAbs), ".docx") {
		return fmt.Errorf("a saída '%s' precisa ter extensão .docx", outputPath)
	}
	if info, err := os.Stat(filepath.Dir(outputAbs)); err != nil {
		return fmt.Errorf("não foi possível acessar a pasta de saída '%s': %w", filepath.Dir(outputAbs), err)
	} else if !info.IsDir() {
		return fmt.Errorf("a pasta de saída '%s' não é um diretório", filepath.Dir(outputAbs))
	}
	if _, err := os.Lstat(outputAbs); err == nil {
		return fmt.Errorf("o arquivo de saída '%s' já existe", outputPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("não foi possível acessar o destino '%s': %w", outputPath, err)
	}

	packages := make([]*mergeDOCXPackage, 0, len(inputPaths))
	closePackages := func() error {
		var errs []error
		for _, input := range packages {
			if err := input.archive.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
	defer func() { _ = closePackages() }()

	for _, inputPath := range inputPaths {
		inputAbs, err := filepath.Abs(inputPath)
		if err != nil {
			return fmt.Errorf("caminho de entrada inválido '%s': %w", inputPath, err)
		}
		inputAbs = filepath.Clean(inputAbs)
		if sameDOCXPath(inputAbs, outputAbs) {
			return fmt.Errorf("o arquivo de saída não pode ser um dos arquivos de entrada: '%s'", inputPath)
		}
		input, err := openMergeDOCXPackage(inputAbs)
		if err != nil {
			return fmt.Errorf("não foi possível abrir '%s': %w", inputPath, err)
		}
		packages = append(packages, input)
	}

	entries, err := mergeDOCXPackages(packages)
	if err != nil {
		return err
	}
	return writeMergedDOCXAtomic(entries, outputAbs)
}

func openMergeDOCXPackage(inputPath string) (*mergeDOCXPackage, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("a entrada não é um arquivo regular")
	}
	if !strings.EqualFold(filepath.Ext(inputPath), ".docx") {
		return nil, fmt.Errorf("a entrada precisa ter extensão .docx")
	}

	archive, err := zip.OpenReader(inputPath)
	if err != nil {
		return nil, fmt.Errorf("arquivo ZIP/DOCX inválido: %w", err)
	}
	input := &mergeDOCXPackage{archive: archive, files: make(map[string]*zip.File)}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = archive.Close()
		}
	}()
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if err := validateOPCPartName(file.Name); err != nil {
			return nil, fmt.Errorf("nome de entrada ZIP inválido '%s': %w", file.Name, err)
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("o DOCX contém uma entrada simbólica '%s'", file.Name)
		}
		if _, duplicate := input.files[file.Name]; duplicate {
			return nil, fmt.Errorf("o DOCX contém a parte duplicada '%s'", file.Name)
		}
		input.files[file.Name] = file
	}

	contentTypesBytes, err := input.readPart(mergeContentTypesPath)
	if err != nil {
		return nil, fmt.Errorf("o DOCX não contém '%s': %w", mergeContentTypesPath, err)
	}
	if err := unmarshalMergeContentTypes(contentTypesBytes, &input.contentTypes); err != nil {
		return nil, fmt.Errorf("não foi possível ler '%s': %w", mergeContentTypesPath, err)
	}
	documentBytes, err := input.readPart(mergeDocumentPath)
	if err != nil {
		return nil, fmt.Errorf("o DOCX não contém '%s': %w", mergeDocumentPath, err)
	}
	input.document, err = parsePackageXML(documentBytes)
	if err != nil {
		return nil, fmt.Errorf("XML principal inválido: %w", err)
	}
	if !isWordElement(input.document.root, "document") {
		return nil, fmt.Errorf("a parte principal não é um documento WordprocessingML")
	}
	if input.document.root.firstElement(wordXMLNamespace, "body") == nil {
		return nil, fmt.Errorf("o documento não contém w:body")
	}

	relsBytes, relsErr := input.readPart(relationshipPartName(mergeDocumentPath))
	if relsErr == nil {
		if err := unmarshalOPCRelationships(relsBytes, &input.documentRels); err != nil {
			return nil, fmt.Errorf("relacionamentos do documento inválidos: %w", err)
		}
	} else if !errors.Is(relsErr, os.ErrNotExist) {
		return nil, fmt.Errorf("não foi possível ler os relacionamentos do documento: %w", relsErr)
	}
	if err := validateRelationshipIDs(input.documentRels); err != nil {
		return nil, fmt.Errorf("relacionamentos do documento inválidos: %w", err)
	}
	if err := validateMergePackageRelationships(input); err != nil {
		return nil, err
	}
	if err := validateMergeDOCXFeatureScope(input); err != nil {
		return nil, err
	}

	input.stylesPath = relatedPartPath(mergeDocumentPath, input.documentRels, "styles")
	if input.stylesPath != "" {
		stylesBytes, err := input.readPart(input.stylesPath)
		if err != nil {
			return nil, fmt.Errorf("não foi possível ler estilos '%s': %w", input.stylesPath, err)
		}
		input.styles, err = parsePackageXML(stylesBytes)
		if err != nil || !isWordElement(input.styles.root, "styles") {
			return nil, fmt.Errorf("a parte de estilos '%s' contém XML inválido", input.stylesPath)
		}
	} else if _, err := input.readPart("word/styles.xml"); err == nil {
		input.stylesPath = "word/styles.xml"
		stylesBytes, _ := input.readPart(input.stylesPath)
		input.styles, err = parsePackageXML(stylesBytes)
		if err != nil {
			return nil, fmt.Errorf("XML de estilos inválido: %w", err)
		}
		if !isWordElement(input.styles.root, "styles") {
			return nil, fmt.Errorf("a parte '%s' não contém definições de estilos WordprocessingML", input.stylesPath)
		}
	}

	input.numberingPath = relatedPartPath(mergeDocumentPath, input.documentRels, "numbering")
	if input.numberingPath != "" {
		numberingBytes, err := input.readPart(input.numberingPath)
		if err != nil {
			return nil, fmt.Errorf("não foi possível ler numeração '%s': %w", input.numberingPath, err)
		}
		input.numbering, err = parsePackageXML(numberingBytes)
		if err != nil || !isWordElement(input.numbering.root, "numbering") {
			return nil, fmt.Errorf("a parte de numeração '%s' contém XML inválido", input.numberingPath)
		}
	}

	closeOnError = false
	return input, nil
}

func validateMergeDOCXFeatureScope(input *mergeDOCXPackage) error {
	for _, relationship := range input.documentRels {
		switch relationshipTypeSuffix(relationship.Type) {
		case "comments", "commentsExtended", "commentsIds":
			return fmt.Errorf("DOCX com comentários não é suportado pela junção")
		case "footnotes":
			return fmt.Errorf("DOCX com notas de rodapé não é suportado pela junção")
		case "endnotes":
			return fmt.Errorf("DOCX com notas de fim não é suportado pela junção")
		case "oleObject", "package", "control":
			return fmt.Errorf("DOCX com objetos incorporados ou controles não é suportado pela junção")
		}
	}
	trackedRevisionElements := map[string]bool{
		"ins": true, "del": true, "delText": true,
		"moveFrom": true, "moveTo": true,
		"moveFromRangeStart": true, "moveFromRangeEnd": true,
		"moveToRangeStart": true, "moveToRangeEnd": true,
		"customXmlInsRangeStart": true, "customXmlInsRangeEnd": true,
		"customXmlDelRangeStart": true, "customXmlDelRangeEnd": true,
		"customXmlMoveFromRangeStart": true, "customXmlMoveFromRangeEnd": true,
		"customXmlMoveToRangeStart": true, "customXmlMoveToRangeEnd": true,
	}
	var visit func(*packageXMLNode) error
	visit = func(node *packageXMLNode) error {
		if node.name.Space == wordXMLNamespace {
			switch node.name.Local {
			case "commentRangeStart", "commentRangeEnd", "commentReference":
				return fmt.Errorf("DOCX com comentários não é suportado pela junção")
			case "footnoteReference":
				return fmt.Errorf("DOCX com notas de rodapé não é suportado pela junção")
			case "endnoteReference":
				return fmt.Errorf("DOCX com notas de fim não é suportado pela junção")
			}
		}
		if node.name.Space == wordXMLNamespace && trackedRevisionElements[node.name.Local] {
			return fmt.Errorf("DOCX com revisões controladas não é suportado pela junção")
		}
		if node.name.Space == wordXMLNamespace && (node.name.Local == "object" || node.name.Local == "control") {
			return fmt.Errorf("DOCX com objetos incorporados ou controles não é suportado pela junção")
		}
		for _, part := range node.parts {
			if part.node != nil {
				if err := visit(part.node); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(input.document.root)
}

func (input *mergeDOCXPackage) readPart(name string) ([]byte, error) {
	file, ok := input.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	return data, nil
}

func validateMergePackageRelationships(input *mergeDOCXPackage) error {
	rootRelsPath := "_rels/.rels"
	rootRelsBytes, err := input.readPart(rootRelsPath)
	if err != nil {
		return fmt.Errorf("o DOCX não contém '%s': %w", rootRelsPath, err)
	}
	var rootRelationships []opcRelationship
	if err := unmarshalOPCRelationships(rootRelsBytes, &rootRelationships); err != nil {
		return fmt.Errorf("relacionamentos do pacote inválidos: %w", err)
	}
	if err := validateRelationshipIDs(rootRelationships); err != nil {
		return fmt.Errorf("relacionamentos do pacote inválidos: %w", err)
	}
	mainPartFound := false
	for _, relationship := range rootRelationships {
		if relationship.TargetMode == "External" {
			continue
		}
		target, _, err := resolveOPCTarget("", relationship.Target)
		if err != nil {
			return fmt.Errorf("relacionamento raiz inválido: %w", err)
		}
		if _, exists := input.files[target]; !exists {
			return fmt.Errorf("relacionamento raiz aponta para a parte ausente '%s'", target)
		}
		if relationshipTypeSuffix(relationship.Type) == "officeDocument" && target == mergeDocumentPath {
			mainPartFound = true
		}
	}
	if !mainPartFound {
		return fmt.Errorf("o pacote não relaciona '%s' como documento principal", mergeDocumentPath)
	}
	for _, relationship := range input.documentRels {
		if relationship.TargetMode == "External" {
			continue
		}
		target, _, err := resolveOPCTarget(mergeDocumentPath, relationship.Target)
		if err != nil {
			return fmt.Errorf("relacionamento '%s' do documento inválido: %w", relationship.ID, err)
		}
		if _, exists := input.files[target]; !exists {
			return fmt.Errorf("relacionamento '%s' aponta para a parte ausente '%s'", relationship.ID, target)
		}
	}
	return nil
}

func validateOPCPartName(name string) error {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || name == ".." {
		return fmt.Errorf("o caminho não é relativo e normalizado")
	}
	return nil
}

func writeMergedDOCXAtomicImpl(entries map[string][]byte, outputPath string) error {
	stage, err := os.CreateTemp(filepath.Dir(outputPath), ".caramel-docx-merge-*.tmp")
	if err != nil {
		return fmt.Errorf("não foi possível preparar o arquivo de saída: %w", err)
	}
	stagePath := stage.Name()
	defer os.Remove(stagePath)

	if err := stage.Chmod(0o644); err != nil {
		_ = stage.Close()
		return fmt.Errorf("não foi possível ajustar as permissões da saída: %w", err)
	}
	writer := zip.NewWriter(stage)
	partNames := make([]string, 0, len(entries))
	for name := range entries {
		if name != mergeContentTypesPath {
			if err := validateOPCPartName(name); err != nil {
				_ = writer.Close()
				_ = stage.Close()
				return fmt.Errorf("nome de parte inválido '%s': %w", name, err)
			}
		}
		partNames = append(partNames, name)
	}
	sort.Strings(partNames)
	for _, name := range partNames {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o644)
		part, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			_ = stage.Close()
			return fmt.Errorf("não foi possível criar a parte '%s': %w", name, err)
		}
		if _, err := part.Write(entries[name]); err != nil {
			_ = writer.Close()
			_ = stage.Close()
			return fmt.Errorf("não foi possível escrever a parte '%s': %w", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		_ = stage.Close()
		return fmt.Errorf("não foi possível finalizar o pacote DOCX: %w", err)
	}
	if err := stage.Sync(); err != nil {
		_ = stage.Close()
		return fmt.Errorf("não foi possível sincronizar o pacote DOCX: %w", err)
	}
	if err := stage.Close(); err != nil {
		return fmt.Errorf("não foi possível fechar o pacote DOCX: %w", err)
	}
	if err := validateMergedDOCX(stagePath); err != nil {
		return fmt.Errorf("o pacote DOCX preparado não passou pela validação: %w", err)
	}
	if err := os.Link(stagePath, outputPath); err != nil {
		if _, statErr := os.Lstat(outputPath); statErr == nil {
			return fmt.Errorf("o arquivo de saída '%s' já existe", outputPath)
		}
		return fmt.Errorf("não foi possível publicar atomicamente '%s': %w", outputPath, err)
	}
	return nil
}

func validateMergedDOCX(path string) error {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer archive.Close()
	seen := make(map[string]bool, len(archive.File))
	parts := make(map[string][]byte, len(archive.File))
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if err := validateOPCPartName(file.Name); err != nil && file.Name != mergeContentTypesPath {
			return err
		}
		if seen[file.Name] {
			return fmt.Errorf("o pacote contém a parte duplicada '%s'", file.Name)
		}
		seen[file.Name] = true
		data, err := copyArchiveEntry(file)
		if err != nil {
			return fmt.Errorf("não foi possível validar '%s': %w", file.Name, err)
		}
		parts[file.Name] = data
	}
	documentBytes, documentExists := parts[mergeDocumentPath]
	contentTypesBytes, contentTypesExists := parts[mergeContentTypesPath]
	if !documentExists || !contentTypesExists {
		return fmt.Errorf("o pacote não contém as partes principais")
	}
	document, err := parsePackageXML(documentBytes)
	if err != nil {
		return err
	}
	if !isWordElement(document.root, "document") || document.root.firstElement(wordXMLNamespace, "body") == nil {
		return fmt.Errorf("o pacote não contém um documento WordprocessingML válido")
	}
	var contentTypes mergeContentTypes
	if err := unmarshalMergeContentTypes(contentTypesBytes, &contentTypes); err != nil {
		return err
	}
	for name := range parts {
		if name == mergeContentTypesPath {
			continue
		}
		if _, err := packagePartContentType(contentTypes, name); err != nil {
			return err
		}
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		var relationships []opcRelationship
		if err := unmarshalOPCRelationships(parts[name], &relationships); err != nil {
			return fmt.Errorf("relacionamentos '%s' inválidos: %w", name, err)
		}
		if err := validateRelationshipIDs(relationships); err != nil {
			return err
		}
		owner := relationshipOwnerPart(name)
		for _, relationship := range relationships {
			if relationship.TargetMode == "External" {
				continue
			}
			target, _, err := resolveOPCTarget(owner, relationship.Target)
			if err != nil {
				return err
			}
			if _, exists := parts[target]; !exists {
				return fmt.Errorf("relacionamento em '%s' aponta para a parte ausente '%s'", name, target)
			}
		}
	}
	return nil
}

func relationshipOwnerPart(relationshipPart string) string {
	directory, base := path.Split(relationshipPart)
	if !strings.HasSuffix(directory, "_rels/") || !strings.HasSuffix(base, ".rels") {
		return ""
	}
	ownerDirectory := strings.TrimSuffix(directory, "_rels/")
	ownerBase := strings.TrimSuffix(base, ".rels")
	if ownerBase == "" {
		return ""
	}
	return path.Join(ownerDirectory, ownerBase)
}
