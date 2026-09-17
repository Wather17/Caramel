package docx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeDOCXPreservesOrderAndRemapsStylesListsAndRelationships(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.docx")
	second := filepath.Join(dir, "second.docx")
	output := filepath.Join(dir, "merged.docx")
	writeMergeDOCXFixture(t, first, "primeiro", "12240", "15840", "FF0000", "first image", "w14", `xmlns:w14="urn:example:w14"`)
	writeMergeDOCXFixture(t, second, "segundo", "16838", "11906", "0000FF", "second image", "w15", `xmlns:w15="urn:example:w15"`)

	if err := MergeDOCX([]string{first, second}, output); err != nil {
		t.Fatalf("MergeDOCX falhou: %v", err)
	}

	text, err := ExtractText(output)
	if err != nil {
		t.Fatalf("não foi possível extrair o texto do resultado: %v", err)
	}
	if firstIndex, secondIndex := strings.Index(text, "primeiro"), strings.Index(text, "segundo"); firstIndex < 0 || secondIndex <= firstIndex {
		t.Fatalf("a ordem dos documentos não foi preservada: %q", text)
	}

	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatalf("a saída não é um DOCX válido: %v", err)
	}
	defer archive.Close()
	parts := make(map[string][]byte)
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("não foi possível ler %s: %v", file.Name, errorsJoin(readErr, closeErr))
		}
		parts[file.Name] = data
	}

	document, err := parsePackageXML(parts[mergeDocumentPath])
	if err != nil {
		t.Fatalf("document.xml do resultado é inválido: %v", err)
	}
	body := document.root.firstElement(wordXMLNamespace, "body")
	if body == nil {
		t.Fatal("o documento juntado não contém w:body")
	}
	if got := attrValueRecursive(body, "paraId"); got != "segundo-paragraph" {
		t.Fatalf("namespace/atributo do segundo documento foi perdido: paraId=%q", got)
	}
	if got, _ := document.root.attributeValue(markupCompatibilityNS, "Ignorable"); !strings.Contains(got, "w14") || !strings.Contains(got, "w15") {
		t.Fatalf("prefixos ignoráveis não foram combinados: %q", got)
	}

	sections := findElements(body, wordXMLNamespace, "sectPr")
	if len(sections) != 2 {
		t.Fatalf("esperava uma seção por documento; recebeu %d", len(sections))
	}
	sectionType, ok := wordAttrValue(sections[0].firstElement(wordXMLNamespace, "type"), "val")
	if !ok || sectionType != "nextPage" {
		t.Fatalf("a fronteira entre documentos não começa nova página: type=%q", sectionType)
	}
	firstPageSize := sections[0].firstElement(wordXMLNamespace, "pgSz")
	lastPageSize := sections[1].firstElement(wordXMLNamespace, "pgSz")
	if width, _ := wordAttrValue(firstPageSize, "w"); width != "12240" {
		t.Errorf("a configuração da primeira origem mudou: page width=%q", width)
	}
	if width, _ := wordAttrValue(lastPageSize, "w"); width != "16838" {
		t.Errorf("a configuração da segunda origem mudou: page width=%q", width)
	}

	styles, err := parsePackageXML(parts["word/styles.xml"])
	if err != nil {
		t.Fatalf("styles.xml do resultado é inválido: %v", err)
	}
	styleIDs := make(map[string]*packageXMLNode)
	for _, style := range styles.root.directElements() {
		if !isWordElement(style, "style") {
			continue
		}
		id, _ := wordAttrValue(style, "styleId")
		styleIDs[id] = style
	}
	for _, id := range []string{"Normal", "CaramelSource2_Normal"} {
		if styleIDs[id] == nil {
			t.Errorf("estilo conflitante ausente: %s", id)
		}
	}
	secondStyleColor := findElements(styleIDs["CaramelSource2_Normal"], wordXMLNamespace, "color")
	if len(secondStyleColor) != 1 {
		t.Fatalf("estilo importado perdeu sua cor: %v", secondStyleColor)
	}
	if color, _ := wordAttrValue(secondStyleColor[0], "val"); color != "0000FF" {
		t.Errorf("cor do estilo importado mudou: %q", color)
	}
	secondStyleSpacing := findElements(styleIDs["CaramelSource2_Normal"], wordXMLNamespace, "spacing")
	if len(secondStyleSpacing) != 1 || wordAttr(secondStyleSpacing[0], "after") != "200" {
		t.Errorf("espaçamento padrão do segundo documento não foi incorporado ao estilo: %v", secondStyleSpacing)
	}
	firstStyleNumbering := findElements(styleIDs["Normal"], wordXMLNamespace, "numId")
	secondStyleNumbering := findElements(styleIDs["CaramelSource2_Normal"], wordXMLNamespace, "numId")
	if len(firstStyleNumbering) != 1 || len(secondStyleNumbering) != 1 {
		t.Fatalf("referências de lista dos estilos não foram preservadas: primeira=%d segunda=%d", len(firstStyleNumbering), len(secondStyleNumbering))
	}
	firstStyleNumberID, _ := wordAttrValue(firstStyleNumbering[0], "val")
	secondStyleNumberID, _ := wordAttrValue(secondStyleNumbering[0], "val")
	if firstStyleNumberID == secondStyleNumberID {
		t.Errorf("referência de lista do estilo importado não foi remapeada: primeiro=%s segundo=%s", firstStyleNumberID, secondStyleNumberID)
	}
	if len(findElements(body, wordXMLNamespace, "pStyle")) < 4 {
		t.Error("parágrafos sem estilo explícito não receberam o estilo padrão de cada origem")
	}
	paragraphStyleIDs := make(map[string]bool)
	for _, style := range findElements(body, wordXMLNamespace, "pStyle") {
		id, _ := wordAttrValue(style, "val")
		paragraphStyleIDs[id] = true
	}
	if !paragraphStyleIDs["Normal"] || !paragraphStyleIDs["CaramelSource2_Normal"] {
		t.Errorf("referências de estilo em conflito não foram remapeadas: %v", paragraphStyleIDs)
	}
	if len(findElements(body, wordXMLNamespace, "rStyle")) == 0 {
		t.Error("estilo de caractere padrão não foi aplicado aos runs importados")
	}
	tables := findElements(body, wordXMLNamespace, "tbl")
	if len(tables) != 2 {
		t.Fatalf("esperava duas tabelas preservadas, recebeu %d", len(tables))
	}
	tableStyles := []string{
		"",
		"",
	}
	for i, table := range tables {
		if tblPr := table.firstElement(wordXMLNamespace, "tblPr"); tblPr != nil {
			if tblStyle := tblPr.firstElement(wordXMLNamespace, "tblStyle"); tblStyle != nil {
				tableStyles[i] = wordAttr(tblStyle, "val")
			}
		}
	}
	if tableStyles[0] != "" || tableStyles[1] != "CaramelSource2_DefaultTable" {
		t.Errorf("estilos padrão das tabelas não foram remapeados: %v", tableStyles)
	}

	numbering, err := parsePackageXML(parts["word/numbering.xml"])
	if err != nil {
		t.Fatalf("numbering.xml do resultado é inválido: %v", err)
	}
	numIDs := make(map[string]bool)
	abstractIDs := make(map[string]bool)
	for _, node := range numbering.root.directElements() {
		if isWordElement(node, "num") {
			id, _ := wordAttrValue(node, "numId")
			if numIDs[id] {
				t.Errorf("numId duplicado após junção: %s", id)
			}
			numIDs[id] = true
		}
		if isWordElement(node, "abstractNum") {
			id, _ := wordAttrValue(node, "abstractNumId")
			if abstractIDs[id] {
				t.Errorf("abstractNumId duplicado após junção: %s", id)
			}
			abstractIDs[id] = true
		}
	}
	if len(numIDs) != 2 || len(abstractIDs) != 2 {
		t.Fatalf("as duas listas não foram mescladas: num=%v abstract=%v", numIDs, abstractIDs)
	}
	if !numIDs[secondStyleNumberID] {
		t.Errorf("o estilo importado aponta para numId inexistente %s", secondStyleNumberID)
	}
	paragraphNumbers := findElements(body, wordXMLNamespace, "numId")
	if len(paragraphNumbers) != 2 {
		t.Fatalf("esperava duas referências de lista, recebeu %d", len(paragraphNumbers))
	}
	for _, reference := range paragraphNumbers {
		id, _ := wordAttrValue(reference, "val")
		if !numIDs[id] {
			t.Errorf("parágrafo aponta para numId inexistente %s", id)
		}
	}
	firstParagraphNumberID, _ := wordAttrValue(paragraphNumbers[0], "val")
	secondParagraphNumberID, _ := wordAttrValue(paragraphNumbers[1], "val")
	if firstParagraphNumberID == secondParagraphNumberID {
		t.Errorf("listas com IDs conflitantes foram mescladas em uma única definição: %s", firstParagraphNumberID)
	}

	var relationships []opcRelationship
	if err := unmarshalOPCRelationships(parts[relationshipPartName(mergeDocumentPath)], &relationships); err != nil {
		t.Fatalf("document.xml.rels do resultado é inválido: %v", err)
	}
	if err := validateRelationshipIDs(relationships); err != nil {
		t.Fatalf("Ids de relacionamentos ficaram duplicados: %v", err)
	}
	relationshipByID := make(map[string]opcRelationship)
	for _, relationship := range relationships {
		relationshipByID[relationship.ID] = relationship
	}
	imageReferences := findElements(body, "http://schemas.openxmlformats.org/drawingml/2006/main", "blip")
	if len(imageReferences) != 2 {
		t.Fatalf("esperava duas referências de imagem, recebeu %d", len(imageReferences))
	}
	imageTargets := make(map[string]string)
	for _, image := range imageReferences {
		relID, _ := image.attributeValue(officeRelationshipsNS, "embed")
		relationship, exists := relationshipByID[relID]
		if !exists {
			t.Errorf("a imagem aponta para um relacionamento ausente: %s", relID)
			continue
		}
		target, _, err := resolveOPCTarget(mergeDocumentPath, relationship.Target)
		if err != nil {
			t.Fatal(err)
		}
		imageTargets[target] = string(parts[target])
	}
	if len(imageTargets) != 2 || imageTargets["word/media/image1.png"] != "first image" || imageTargets["word/media/caramel_src02_image1.png"] != "second image" {
		t.Errorf("mídias com mesmo nome não foram preservadas: %v", imageTargets)
	}
	headerTargets := make(map[string]string)
	for _, section := range sections {
		headerReference := section.firstElement(wordXMLNamespace, "headerReference")
		if headerReference == nil {
			t.Fatal("configuração da seção perdeu a referência de cabeçalho")
		}
		relID, _ := headerReference.attributeValue(officeRelationshipsNS, "id")
		relationship, exists := relationshipByID[relID]
		if !exists {
			t.Fatalf("o cabeçalho aponta para relacionamento ausente: %s", relID)
		}
		target, _, err := resolveOPCTarget(mergeDocumentPath, relationship.Target)
		if err != nil {
			t.Fatal(err)
		}
		headerTargets[target] = string(parts[target])
	}
	if len(headerTargets) != 2 || !strings.Contains(headerTargets["word/header1.xml"], "cabeçalho-primeiro") || !strings.Contains(headerTargets["word/caramel_src02_header1.xml"], "cabeçalho-segundo") {
		t.Errorf("cabeçalhos de seção não foram mantidos por origem: %v", headerTargets)
	}
	secondHeader, err := parsePackageXML(parts["word/caramel_src02_header1.xml"])
	if err != nil {
		t.Fatalf("cabeçalho importado ficou inválido: %v", err)
	}
	if styles := findElements(secondHeader.root, wordXMLNamespace, "pStyle"); len(styles) != 2 || wordAttr(styles[0], "val") != "CaramelSource2_Normal" || wordAttr(styles[1], "val") != "CaramelSource2_Normal" {
		t.Errorf("estilo do cabeçalho importado não foi remapeado: %v", styles)
	}
	if numbers := findElements(secondHeader.root, wordXMLNamespace, "numId"); len(numbers) != 1 || wordAttr(numbers[0], "val") != secondStyleNumberID {
		t.Errorf("lista do cabeçalho importado não foi remapeada: %v", numbers)
	}
	secondHeaderRels, err := unmarshalRelationshipsFromPackage(parts, "word/_rels/caramel_src02_header1.xml.rels")
	if err != nil {
		t.Fatal(err)
	}
	if len(secondHeaderRels) != 1 {
		t.Fatalf("relacionamentos internos do cabeçalho foram perdidos: %v", secondHeaderRels)
	}
	headerImage, _, err := resolveOPCTarget("word/caramel_src02_header1.xml", secondHeaderRels[0].Target)
	if err != nil || string(parts[headerImage]) != "header second image" {
		t.Errorf("imagem do cabeçalho foi perdida: path=%s err=%v", headerImage, err)
	}

	if len(findElements(body, wordXMLNamespace, "hyperlink")) != 2 {
		t.Fatal("links das duas origens não foram mantidos")
	}
}

func TestMergeDOCXRejectsInvalidInputAndNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.docx")
	second := filepath.Join(dir, "broken.docx")
	output := filepath.Join(dir, "merged.docx")
	writeMergeDOCXFixture(t, first, "primeiro", "12240", "15840", "FF0000", "image", "w14", `xmlns:w14="urn:example:w14"`)
	if err := os.WriteFile(second, []byte("não é DOCX"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MergeDOCX([]string{first, second}, output); err == nil {
		t.Fatal("MergeDOCX deveria rejeitar um pacote de entrada inválido")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatalf("falha de entrada deixou uma saída parcial: stat err=%v", err)
	}

	if err := os.WriteFile(output, []byte("manter"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MergeDOCX([]string{first, first}, output); err == nil {
		t.Fatal("MergeDOCX deveria rejeitar destino existente")
	}
	if data, err := os.ReadFile(output); err != nil || string(data) != "manter" {
		t.Fatalf("destino existente foi alterado: data=%q err=%v", data, err)
	}
	if err := MergeDOCX([]string{first, second}, first); err == nil {
		t.Fatal("MergeDOCX deveria rejeitar destino igual a uma entrada")
	}
	if _, err := ExtractText(first); err != nil {
		t.Fatalf("a entrada foi alterada: %v", err)
	}
}

func TestMergeDOCXRequiresTwoInputsAndOutputPath(t *testing.T) {
	if err := MergeDOCX([]string{"one.docx"}, "out.docx"); err == nil {
		t.Fatal("MergeDOCX deveria exigir ao menos dois arquivos")
	}
	if err := MergeDOCX([]string{"one.docx", "two.docx"}, "  "); err == nil {
		t.Fatal("MergeDOCX deveria exigir o destino")
	}
}

func TestMergeDOCXRejectsUnresolvedNumberingReferences(t *testing.T) {
	paragraph, err := parsePackageXMLNode(`<w:p xmlns:w="` + wordXMLNamespace + `"><w:pPr><w:numPr><w:numId w:val="7"/></w:numPr></w:pPr></w:p>`)
	if err != nil {
		t.Fatal(err)
	}
	if err := mergeWordNumberingReferences(paragraph, map[string]string{}, map[string]string{}, map[string]string{}); err == nil {
		t.Fatal("uma referência de numeração sem definição deveria ser rejeitada")
	}

	noNumbering, err := parsePackageXMLNode(`<w:p xmlns:w="` + wordXMLNamespace + `"><w:pPr><w:numPr><w:numId w:val="0"/></w:numPr></w:pPr></w:p>`)
	if err != nil {
		t.Fatal(err)
	}
	if err := mergeWordNumberingReferences(noNumbering, map[string]string{}, map[string]string{}, map[string]string{}); err != nil {
		t.Fatalf("numId 0 deveria continuar representando ausência de numeração: %v", err)
	}
}

func TestMergeDOCXPartNameAndNumberingIDValidation(t *testing.T) {
	entries := map[string][]byte{"word/styles.xml": nil, "word/caramel_styles.xml": nil}
	if got := uniqueMergePartName(entries, "word/styles.xml"); got != "word/caramel_styles_2.xml" {
		t.Fatalf("nome de parte não foi alocado sem colisão: %s", got)
	}

	numbering, err := parsePackageXMLNode(`<w:abstractNum xmlns:w="` + wordXMLNamespace + `" w:abstractNumId="invalid"/>`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := numberingDefinitionIdentityMap([]*packageXMLNode{numbering}, "abstractNum", "abstractNumId", map[string]bool{}); err == nil {
		t.Fatal("um ID não numérico deveria ser rejeitado")
	}
}

func TestMergeDOCXRejectsFeaturesOutsideSupportedScope(t *testing.T) {
	tracked, err := parsePackageXML([]byte(`<w:document xmlns:w="` + wordXMLNamespace + `"><w:body><w:ins><w:p/></w:ins></w:body></w:document>`))
	if err != nil {
		t.Fatal(err)
	}
	input := &mergeDOCXPackage{document: tracked}
	if err := validateMergeDOCXFeatureScope(input); err == nil || !strings.Contains(err.Error(), "revisões controladas") {
		t.Fatalf("revisões controladas deveriam ser rejeitadas claramente: %v", err)
	}

	plain, err := parsePackageXML([]byte(`<w:document xmlns:w="` + wordXMLNamespace + `"><w:body/></w:document>`))
	if err != nil {
		t.Fatal(err)
	}
	input = &mergeDOCXPackage{
		document:     plain,
		documentRels: []opcRelationship{{ID: "rIdComments", Type: officeRelationshipsNS + "/comments", Target: "comments.xml"}},
	}
	if err := validateMergeDOCXFeatureScope(input); err == nil || !strings.Contains(err.Error(), "comentários") {
		t.Fatalf("comentários deveriam ser rejeitados claramente: %v", err)
	}
}

func writeMergeDOCXFixture(t *testing.T, destination, text, pageWidth, pageHeight, styleColor, imageData, ignorablePrefix, extraNamespace string) {
	t.Helper()
	file, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	document := fmt.Sprintf(`<?xml version="1.0"?><w:document xmlns:w="%s" xmlns:r="%s" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:mc="%s" %s mc:Ignorable="%s"><w:body><w:p %s:paraId="%s-paragraph"><w:pPr><w:pStyle w:val="Normal"/><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>%s</w:t></w:r></w:p><w:p><w:r><w:drawing><a:blip r:embed="rIdImage"/></w:drawing></w:r></w:p><w:p><w:hyperlink r:id="rIdLink"><w:r><w:t>link-%s</w:t></w:r></w:hyperlink></w:p><w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/></w:tblPr><w:tblGrid><w:gridCol w:w="1000"/></w:tblGrid><w:tr><w:tc><w:p><w:r><w:t>célula</w:t></w:r></w:p></w:tc></w:tr></w:tbl><w:p><w:r><w:t>sem estilo explícito</w:t></w:r></w:p><w:sectPr><w:headerReference w:type="default" r:id="rIdHeader"/><w:pgSz w:w="%s" w:h="%s"/><w:pgMar w:top="720" w:right="720" w:bottom="720" w:left="720"/></w:sectPr></w:body></w:document>`, wordXMLNamespace, officeRelationshipsNS, markupCompatibilityNS, extraNamespace, ignorablePrefix, ignorablePrefix, text, text, text, pageWidth, pageHeight)
	styles := fmt.Sprintf(`<w:styles xmlns:w="%s"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Arial-%s"/><w:color w:val="00FF00"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="200"/></w:pPr></w:pPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr><w:rPr><w:color w:val="%s"/></w:rPr></w:style><w:style w:type="character" w:default="1" w:styleId="DefaultChar"><w:name w:val="Default Char"/></w:style><w:style w:type="table" w:default="1" w:styleId="DefaultTable"><w:name w:val="Default Table"/></w:style></w:styles>`, wordXMLNamespace, text, styleColor)
	numbering := fmt.Sprintf(`<w:numbering xmlns:w="%s"><w:abstractNum w:abstractNumId="0"><w:multiLevelType w:val="singleLevel"/><w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/><w:lvlText w:val="%%1."/></w:lvl></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num></w:numbering>`, wordXMLNamespace)
	entries := map[string]string{
		"[Content_Types].xml":          fmt.Sprintf(`<Types xmlns="%s"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/><Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/><Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/><Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`, opcContentTypesNamespace),
		"_rels/.rels":                  fmt.Sprintf(`<Relationships xmlns="%s"><Relationship Id="rIdRoot" Type="%s/officeDocument" Target="word/document.xml"/></Relationships>`, opcRelationshipsNamespace, officeRelationshipsNS),
		"word/document.xml":            document,
		"word/_rels/document.xml.rels": fmt.Sprintf(`<Relationships xmlns="%s"><Relationship Id="rIdStyles" Type="%s/styles" Target="styles.xml"/><Relationship Id="rIdNumbering" Type="%s/numbering" Target="numbering.xml"/><Relationship Id="rIdImage" Type="%s/image" Target="media/image1.png"/><Relationship Id="rIdLink" Type="%s/hyperlink" Target="https://example.com/%s" TargetMode="External"/><Relationship Id="rIdSettings" Type="%s/settings" Target="settings.xml"/><Relationship Id="rIdHeader" Type="%s/header" Target="header1.xml"/></Relationships>`, opcRelationshipsNamespace, officeRelationshipsNS, officeRelationshipsNS, officeRelationshipsNS, officeRelationshipsNS, text, officeRelationshipsNS, officeRelationshipsNS),
		"word/styles.xml":              styles,
		"word/numbering.xml":           numbering,
		"word/settings.xml":            fmt.Sprintf(`<w:settings xmlns:w="%s"/>`, wordXMLNamespace),
		"word/media/image1.png":        imageData,
		"word/header1.xml":             fmt.Sprintf(`<w:hdr xmlns:w="%s" xmlns:r="%s" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><w:p><w:pPr><w:pStyle w:val="Normal"/><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>cabeçalho-%s</w:t></w:r></w:p><w:p><w:r><w:drawing><a:blip r:embed="rIdHeaderImage"/></w:drawing></w:r></w:p></w:hdr>`, wordXMLNamespace, officeRelationshipsNS, text),
		"word/_rels/header1.xml.rels":  fmt.Sprintf(`<Relationships xmlns="%s"><Relationship Id="rIdHeaderImage" Type="%s/image" Target="media/header.png"/></Relationships>`, opcRelationshipsNamespace, officeRelationshipsNS),
		"word/media/header.png":        "header " + imageData,
	}
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func unmarshalRelationshipsFromPackage(parts map[string][]byte, partName string) ([]opcRelationship, error) {
	var relationships []opcRelationship
	if err := unmarshalOPCRelationships(parts[partName], &relationships); err != nil {
		return nil, err
	}
	return relationships, nil
}

func findElements(root *packageXMLNode, namespace, local string) []*packageXMLNode {
	var found []*packageXMLNode
	var walk func(*packageXMLNode)
	walk = func(node *packageXMLNode) {
		if node == nil {
			return
		}
		if node.name.Space == namespace && node.name.Local == local {
			found = append(found, node)
		}
		for _, part := range node.parts {
			if part.node != nil {
				walk(part.node)
			}
		}
	}
	walk(root)
	return found
}

func attrValueRecursive(root *packageXMLNode, local string) string {
	var found string
	var walk func(*packageXMLNode)
	walk = func(node *packageXMLNode) {
		if node == nil || found != "" {
			return
		}
		if value, ok := node.attributeValue("urn:example:w15", local); ok {
			found = value
		}
		for _, part := range node.parts {
			if part.node != nil {
				walk(part.node)
			}
		}
	}
	walk(root)
	return found
}

func wordAttr(node *packageXMLNode, local string) string {
	value, _ := wordAttrValue(node, local)
	return value
}

func errorsJoin(first, second error) error {
	if first != nil {
		return first
	}
	return second
}
