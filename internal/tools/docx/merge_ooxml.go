package docx

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
)

const (
	opcRelationshipsNamespace = "http://schemas.openxmlformats.org/package/2006/relationships"
	opcContentTypesNamespace  = "http://schemas.openxmlformats.org/package/2006/content-types"
	officeRelationshipsNS     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	markupCompatibilityNS     = "http://schemas.openxmlformats.org/markup-compatibility/2006"
)

type opcRelationship struct {
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr,omitempty"`
}

type opcRelationships struct {
	XMLName       xml.Name          `xml:"http://schemas.openxmlformats.org/package/2006/relationships Relationships"`
	Relationships []opcRelationship `xml:"Relationship"`
}

type mergeContentTypeDefault struct {
	Extension   string `xml:"Extension,attr"`
	ContentType string `xml:"ContentType,attr"`
}

type mergeContentTypeOverride struct {
	PartName    string `xml:"PartName,attr"`
	ContentType string `xml:"ContentType,attr"`
}

type mergeContentTypes struct {
	XMLName   xml.Name                   `xml:"http://schemas.openxmlformats.org/package/2006/content-types Types"`
	Defaults  []mergeContentTypeDefault  `xml:"Default"`
	Overrides []mergeContentTypeOverride `xml:"Override"`
}

func unmarshalOPCRelationships(data []byte, relationships *[]opcRelationship) error {
	var document opcRelationships
	if err := xml.Unmarshal(data, &document); err != nil {
		return err
	}
	if document.XMLName.Local != "Relationships" || document.XMLName.Space != opcRelationshipsNamespace {
		return fmt.Errorf("raiz Relationships ausente")
	}
	*relationships = document.Relationships
	return nil
}

func marshalOPCRelationships(relationships []opcRelationship) ([]byte, error) {
	document := opcRelationships{
		XMLName:       xml.Name{Space: opcRelationshipsNamespace, Local: "Relationships"},
		Relationships: relationships,
	}
	return xml.Marshal(document)
}

func validateRelationshipIDs(relationships []opcRelationship) error {
	seen := make(map[string]bool, len(relationships))
	for _, relationship := range relationships {
		if relationship.ID == "" || relationship.Type == "" || relationship.Target == "" {
			return fmt.Errorf("relacionamento sem Id, Type ou Target")
		}
		if seen[relationship.ID] {
			return fmt.Errorf("Id duplicado '%s'", relationship.ID)
		}
		seen[relationship.ID] = true
	}
	return nil
}

func unmarshalMergeContentTypes(data []byte, contentTypes *mergeContentTypes) error {
	if err := xml.Unmarshal(data, contentTypes); err != nil {
		return err
	}
	if contentTypes.XMLName.Local != "Types" || contentTypes.XMLName.Space != opcContentTypesNamespace {
		return fmt.Errorf("raiz Types ausente")
	}
	return nil
}

func relatedPartPath(owner string, relationships []opcRelationship, relationType string) string {
	for _, relationship := range relationships {
		if relationship.TargetMode == "External" || relationshipTypeSuffix(relationship.Type) != relationType {
			continue
		}
		part, _, err := resolveOPCTarget(owner, relationship.Target)
		if err == nil {
			return part
		}
	}
	return ""
}

func relationshipTypeSuffix(relationType string) string {
	return path.Base(strings.TrimSuffix(relationType, "/"))
}

func relationshipPartName(ownerPart string) string {
	directory, base := path.Split(ownerPart)
	return path.Join(directory, "_rels", base+".rels")
}

func resolveOPCTarget(ownerPart, target string) (partName, suffix string, err error) {
	reference, err := url.Parse(target)
	if err != nil {
		return "", "", fmt.Errorf("URI de relacionamento inválida '%s': %w", target, err)
	}
	if reference.IsAbs() || reference.Scheme != "" || reference.Opaque != "" || reference.RawQuery != "" {
		return "", "", fmt.Errorf("URI interna não suportada '%s'", target)
	}
	targetPath := reference.Path
	if strings.Contains(targetPath, "\\") {
		return "", "", fmt.Errorf("URI contém separador inválido")
	}
	if strings.HasPrefix(targetPath, "/") {
		targetPath = strings.TrimPrefix(targetPath, "/")
	} else {
		targetPath = path.Join(path.Dir(ownerPart), targetPath)
	}
	targetPath = path.Clean(targetPath)
	if err := validateOPCPartName(targetPath); err != nil {
		return "", "", err
	}
	suffix = ""
	if reference.Fragment != "" {
		suffix = "#" + reference.EscapedFragment()
	}
	return targetPath, suffix, nil
}

func relativeOPCTarget(ownerPart, targetPart string) string {
	from := strings.Split(strings.Trim(path.Dir(ownerPart), "/"), "/")
	to := strings.Split(strings.Trim(targetPart, "/"), "/")
	if len(from) == 1 && from[0] == "." {
		from = nil
	}
	common := 0
	for common < len(from) && common < len(to) && from[common] == to[common] {
		common++
	}
	parts := make([]string, 0, len(from)-common+len(to)-common)
	for range len(from) - common {
		parts = append(parts, "..")
	}
	parts = append(parts, to[common:]...)
	relative := strings.Join(parts, "/")
	return (&url.URL{Path: relative}).EscapedPath()
}

func packagePartContentType(contentTypes mergeContentTypes, partName string) (string, error) {
	for _, override := range contentTypes.Overrides {
		if strings.TrimPrefix(override.PartName, "/") == partName {
			return override.ContentType, nil
		}
	}
	extension := strings.TrimPrefix(path.Ext(partName), ".")
	for _, defaultType := range contentTypes.Defaults {
		if strings.EqualFold(defaultType.Extension, extension) {
			return defaultType.ContentType, nil
		}
	}
	return "", fmt.Errorf("o tipo de conteúdo de '%s' não está declarado", partName)
}

func marshalMergeContentTypes(contentTypes map[string]string) ([]byte, error) {
	partNames := make([]string, 0, len(contentTypes))
	for partName := range contentTypes {
		partNames = append(partNames, partName)
	}
	sort.Strings(partNames)
	document := struct {
		XMLName   xml.Name                   `xml:"http://schemas.openxmlformats.org/package/2006/content-types Types"`
		Overrides []mergeContentTypeOverride `xml:"Override"`
	}{
		XMLName: xml.Name{Space: opcContentTypesNamespace, Local: "Types"},
	}
	for _, partName := range partNames {
		document.Overrides = append(document.Overrides, mergeContentTypeOverride{
			PartName:    "/" + partName,
			ContentType: contentTypes[partName],
		})
	}
	return xml.Marshal(document)
}

func isSingletonDocumentRelationship(relationshipType string) bool {
	switch relationshipTypeSuffix(relationshipType) {
	case "styles", "stylesWithEffects", "numbering", "settings", "webSettings", "fontTable", "theme":
		return true
	default:
		return false
	}
}

type mergePartKey struct {
	source int
	name   string
}

type mergePartCopier struct {
	packages      []*mergeDOCXPackage
	entries       map[string][]byte
	contentTypes  map[string]string
	copiedParts   map[mergePartKey]string
	styleMaps     []map[string]string
	numberingMaps []mergedNumberingMaps
}

func newMergePartCopier(packages []*mergeDOCXPackage) (*mergePartCopier, error) {
	copier := &mergePartCopier{
		packages:     packages,
		entries:      make(map[string][]byte),
		contentTypes: make(map[string]string),
		copiedParts:  make(map[mergePartKey]string),
	}
	master := packages[0]
	for name := range master.files {
		if name == mergeContentTypesPath {
			continue
		}
		contentType, err := packagePartContentType(master.contentTypes, name)
		if err != nil {
			return nil, fmt.Errorf("não foi possível determinar o tipo de '%s': %w", name, err)
		}
		data, err := master.readPart(name)
		if err != nil {
			return nil, fmt.Errorf("não foi possível copiar a parte '%s': %w", name, err)
		}
		copier.entries[name] = data
		copier.contentTypes[name] = contentType
		copier.copiedParts[mergePartKey{source: 0, name: name}] = name
	}
	return copier, nil
}

func (copier *mergePartCopier) copyRelatedPart(sourceIndex int, sourcePart string) (string, error) {
	key := mergePartKey{source: sourceIndex, name: sourcePart}
	if outputPart, exists := copier.copiedParts[key]; exists {
		return outputPart, nil
	}
	source := copier.packages[sourceIndex]
	if err := validateOPCPartName(sourcePart); err != nil {
		return "", err
	}
	if _, exists := source.files[sourcePart]; !exists {
		return "", fmt.Errorf("relacionamento aponta para a parte ausente '%s'", sourcePart)
	}
	outputPart := copier.uniqueOutputPartName(sourceIndex, sourcePart)
	// Record the mapping before following relationships so cyclic part graphs terminate.
	copier.copiedParts[key] = outputPart

	contentType, err := packagePartContentType(source.contentTypes, sourcePart)
	if err != nil {
		return "", err
	}
	data, err := source.readPart(sourcePart)
	if err != nil {
		return "", fmt.Errorf("não foi possível ler a parte '%s': %w", sourcePart, err)
	}
	data, err = copier.remapRelatedWordPart(sourceIndex, contentType, data)
	if err != nil {
		return "", err
	}
	copier.entries[outputPart] = data
	copier.contentTypes[outputPart] = contentType

	sourceRelsPath := relationshipPartName(sourcePart)
	if _, exists := source.files[sourceRelsPath]; !exists {
		return outputPart, nil
	}
	relsBytes, err := source.readPart(sourceRelsPath)
	if err != nil {
		return "", fmt.Errorf("não foi possível ler '%s': %w", sourceRelsPath, err)
	}
	var relationships []opcRelationship
	if err := unmarshalOPCRelationships(relsBytes, &relationships); err != nil {
		return "", fmt.Errorf("relacionamentos de '%s' são inválidos: %w", sourcePart, err)
	}
	if err := validateRelationshipIDs(relationships); err != nil {
		return "", fmt.Errorf("relacionamentos de '%s' são inválidos: %w", sourcePart, err)
	}
	for i := range relationships {
		relationship := &relationships[i]
		if relationship.TargetMode == "External" {
			continue
		}
		targetPart, suffix, err := resolveOPCTarget(sourcePart, relationship.Target)
		if err != nil {
			return "", fmt.Errorf("relacionamento de '%s' inválido: %w", sourcePart, err)
		}
		outputTarget, err := copier.copyRelatedPart(sourceIndex, targetPart)
		if err != nil {
			return "", err
		}
		relationship.Target = relativeOPCTarget(outputPart, outputTarget) + suffix
	}
	outputRelsPath := relationshipPartName(outputPart)
	if _, collision := copier.entries[outputRelsPath]; collision {
		return "", fmt.Errorf("colisão ao preparar relacionamentos '%s'", outputRelsPath)
	}
	outputRels, err := marshalOPCRelationships(relationships)
	if err != nil {
		return "", err
	}
	relsContentType, err := packagePartContentType(source.contentTypes, sourceRelsPath)
	if err != nil {
		return "", err
	}
	copier.entries[outputRelsPath] = outputRels
	copier.contentTypes[outputRelsPath] = relsContentType
	return outputPart, nil
}

func (copier *mergePartCopier) remapRelatedWordPart(sourceIndex int, contentType string, data []byte) ([]byte, error) {
	if !strings.HasSuffix(contentType, ".header+xml") && !strings.HasSuffix(contentType, ".footer+xml") {
		return data, nil
	}
	document, err := parsePackageXML(data)
	if err != nil {
		return nil, fmt.Errorf("parte de cabeçalho/rodapé inválida: %w", err)
	}
	if !isWordElement(document.root, "hdr") && !isWordElement(document.root, "ftr") {
		return nil, fmt.Errorf("parte de cabeçalho/rodapé contém uma raiz WordprocessingML inválida")
	}
	if sourceIndex < len(copier.styleMaps) {
		if sourceIndex < len(copier.packages) {
			defaults := defaultWordStyles(copier.packages[sourceIndex].styles, copier.styleMaps[sourceIndex])
			if err := addDefaultStyleReferences(document.root, defaults); err != nil {
				return nil, err
			}
		}
		if err := mergeWordStyleReferences(document.root, copier.styleMaps[sourceIndex]); err != nil {
			return nil, err
		}
	}
	if sourceIndex < len(copier.numberingMaps) {
		mapping := copier.numberingMaps[sourceIndex]
		if err := mergeWordNumberingReferences(document.root, mapping.numbers, mapping.abstracts, mapping.pictures); err != nil {
			return nil, err
		}
	}
	return document.bytes(), nil
}

func (copier *mergePartCopier) uniqueOutputPartName(sourceIndex int, sourcePart string) string {
	directory, base := path.Split(sourcePart)
	for suffix := 1; ; suffix++ {
		candidateBase := fmt.Sprintf("caramel_src%02d_%s", sourceIndex+1, base)
		if suffix > 1 {
			candidateBase = fmt.Sprintf("caramel_src%02d_%d_%s", sourceIndex+1, suffix, base)
		}
		candidate := path.Join(directory, candidateBase)
		if _, exists := copier.entries[candidate]; !exists {
			return candidate
		}
	}
}

func buildMergedDocumentRelationships(packages []*mergeDOCXPackage, copier *mergePartCopier, masterRelationships []opcRelationship) ([]opcRelationship, []map[string]string, error) {
	merged := append([]opcRelationship(nil), masterRelationships...)
	used := make(map[string]bool, len(merged))
	referenceMaps := make([]map[string]string, len(packages))
	for _, relationship := range merged {
		used[relationship.ID] = true
	}
	relationshipNumber := 1
	allocateID := func() string {
		for {
			candidate := fmt.Sprintf("rIdCaramelMerge%d", relationshipNumber)
			relationshipNumber++
			if !used[candidate] {
				used[candidate] = true
				return candidate
			}
		}
	}
	for sourceIndex, source := range packages {
		mapping := make(map[string]string, len(source.documentRels))
		for _, relationship := range source.documentRels {
			if sourceIndex == 0 {
				mapping[relationship.ID] = relationship.ID
				continue
			}
			if isSingletonDocumentRelationship(relationship.Type) {
				mapping[relationship.ID] = ""
				continue
			}
			outputRelationship := relationship
			outputRelationship.ID = allocateID()
			mapping[relationship.ID] = outputRelationship.ID
			if relationship.TargetMode != "External" {
				targetPart, suffix, err := resolveOPCTarget(mergeDocumentPath, relationship.Target)
				if err != nil {
					return nil, nil, fmt.Errorf("relacionamento '%s' inválido no documento %d: %w", relationship.ID, sourceIndex+1, err)
				}
				outputTarget, err := copier.copyRelatedPart(sourceIndex, targetPart)
				if err != nil {
					return nil, nil, fmt.Errorf("não foi possível incluir a parte relacionada '%s': %w", targetPart, err)
				}
				outputRelationship.Target = relativeOPCTarget(mergeDocumentPath, outputTarget) + suffix
			}
			merged = append(merged, outputRelationship)
		}
		referenceMaps[sourceIndex] = mapping
	}
	return merged, referenceMaps, nil
}

func buildMergedStyles(packages []*mergeDOCXPackage) (*packageXMLDocument, []map[string]string, string, error) {
	styleMaps := make([]map[string]string, len(packages))
	used := make(map[string]bool)
	for sourceIndex, input := range packages {
		mapping := make(map[string]string, len(used))
		for styleID := range used {
			mapping[styleID] = styleID
		}
		if input.styles != nil {
			seenSource := make(map[string]bool)
			for _, style := range input.styles.root.directElements() {
				if !isWordElement(style, "style") {
					continue
				}
				styleID, _ := wordAttrValue(style, "styleId")
				if styleID == "" {
					continue
				}
				if seenSource[styleID] {
					return nil, nil, "", fmt.Errorf("o documento %d contém o estilo duplicado '%s'", sourceIndex+1, styleID)
				}
				seenSource[styleID] = true
				newID := styleID
				if used[newID] {
					newID = uniqueMergedStyleID(sourceIndex+1, styleID, used)
				}
				mapping[styleID] = newID
				used[newID] = true
			}
		}
		styleMaps[sourceIndex] = mapping
	}

	var merged *packageXMLDocument
	stylePath := ""
	for sourceIndex, input := range packages {
		if input.styles == nil {
			continue
		}
		if merged == nil {
			merged = &packageXMLDocument{
				prefix: append([]byte(nil), input.styles.prefix...),
				root:   input.styles.root.clone(),
				suffix: append([]byte(nil), input.styles.suffix...),
			}
			stylePath = input.stylesPath
			continue
		}
		if err := ensureMergeNamespace(merged.root, input.styles.root); err != nil {
			return nil, nil, "", err
		}
		for _, style := range input.styles.root.directElements() {
			if !isWordElement(style, "style") {
				continue
			}
			copy := style.clone()
			styleID, _ := wordAttrValue(copy, "styleId")
			if mapped := styleMaps[sourceIndex][styleID]; mapped != "" && mapped != styleID {
				if err := setWordAttr(copy, "styleId", mapped); err != nil {
					return nil, nil, "", err
				}
			}
			if err := mergeWordStyleReferences(copy, styleMaps[sourceIndex]); err != nil {
				return nil, nil, "", err
			}
			if err := mergeDefaultStyleProperties(copy, input.styles); err != nil {
				return nil, nil, "", err
			}
			if defaultValue, _ := wordAttrValue(copy, "default"); defaultValue == "1" || strings.EqualFold(defaultValue, "true") || strings.EqualFold(defaultValue, "on") {
				if err := setWordAttr(copy, "default", "0"); err != nil {
					return nil, nil, "", err
				}
			}
			insertPackageXMLChildBeforeWordElement(merged.root, copy, "tableStyles", "extLst")
		}
	}
	return merged, styleMaps, stylePath, nil
}

func uniqueMergePartName(entries map[string][]byte, preferred string) string {
	if _, exists := entries[preferred]; !exists {
		return preferred
	}
	directory, base := path.Split(preferred)
	extension := path.Ext(base)
	stem := strings.TrimSuffix(base, extension)
	for suffix := 1; ; suffix++ {
		candidateBase := fmt.Sprintf("caramel_%s%s", stem, extension)
		if suffix > 1 {
			candidateBase = fmt.Sprintf("caramel_%s_%d%s", stem, suffix, extension)
		}
		candidate := path.Join(directory, candidateBase)
		if _, exists := entries[candidate]; !exists {
			return candidate
		}
	}
}

func remapMergedStylesNumberingReferences(styles *packageXMLDocument, packages []*mergeDOCXPackage, styleMaps []map[string]string, numberingMaps []mergedNumberingMaps) error {
	if styles == nil {
		return nil
	}
	mergedStyles := make(map[string]*packageXMLNode)
	for _, style := range styles.root.directElements() {
		if !isWordElement(style, "style") {
			continue
		}
		styleID, _ := wordAttrValue(style, "styleId")
		if styleID != "" {
			mergedStyles[styleID] = style
		}
	}
	for sourceIndex, input := range packages {
		if input.styles == nil || sourceIndex >= len(numberingMaps) {
			continue
		}
		for _, sourceStyle := range input.styles.root.directElements() {
			if !isWordElement(sourceStyle, "style") {
				continue
			}
			styleID, _ := wordAttrValue(sourceStyle, "styleId")
			mergedID := styleMaps[sourceIndex][styleID]
			style := mergedStyles[mergedID]
			if style == nil {
				continue
			}
			mapping := numberingMaps[sourceIndex]
			if err := mergeWordNumberingReferences(style, mapping.numbers, mapping.abstracts, mapping.pictures); err != nil {
				return fmt.Errorf("referência de numeração no estilo '%s' inválida: %w", styleID, err)
			}
		}
	}
	return nil
}

func uniqueMergedStyleID(sourceNumber int, styleID string, used map[string]bool) string {
	base := fmt.Sprintf("CaramelSource%d_%s", sourceNumber, styleID)
	candidate := base
	for suffix := 2; used[candidate]; suffix++ {
		candidate = fmt.Sprintf("%s_%d", base, suffix)
	}
	return candidate
}

func mergeDefaultStyleProperties(style *packageXMLNode, sourceStyles *packageXMLDocument) error {
	if sourceStyles == nil {
		return nil
	}
	defaultValue, _ := wordAttrValue(style, "default")
	if defaultValue != "1" && !strings.EqualFold(defaultValue, "true") && !strings.EqualFold(defaultValue, "on") {
		return nil
	}
	docDefaults := sourceStyles.root.firstElement(wordXMLNamespace, "docDefaults")
	if docDefaults == nil {
		return nil
	}
	styleType, _ := wordAttrValue(style, "type")
	if styleType != "paragraph" && styleType != "character" {
		return nil
	}
	if styleType == "paragraph" {
		if paragraphDefaults := docDefaults.firstElement(wordXMLNamespace, "pPrDefault"); paragraphDefaults != nil {
			if properties := paragraphDefaults.firstElement(wordXMLNamespace, "pPr"); properties != nil {
				if err := mergeDefaultPropertiesIntoStyle(style, "pPr", properties); err != nil {
					return err
				}
			}
		}
	}
	if runDefaults := docDefaults.firstElement(wordXMLNamespace, "rPrDefault"); runDefaults != nil {
		if properties := runDefaults.firstElement(wordXMLNamespace, "rPr"); properties != nil {
			if err := mergeDefaultPropertiesIntoStyle(style, "rPr", properties); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeDefaultPropertiesIntoStyle(style *packageXMLNode, propertyName string, defaults *packageXMLNode) error {
	properties := style.firstElement(wordXMLNamespace, propertyName)
	if properties == nil {
		fragment := `<w:` + propertyName + ` xmlns:w="` + wordXMLNamespace + `"/>`
		var err error
		properties, err = parsePackageXMLNode(fragment)
		if err != nil {
			return err
		}
		if propertyName == "pPr" {
			insertPackageXMLChildBeforeWordElement(style, properties, "rPr", "tblPr", "trPr", "tcPr", "extLst")
		} else {
			insertPackageXMLChildBeforeWordElement(style, properties, "tblPr", "trPr", "tcPr", "extLst")
		}
	}
	defaultElements := defaults.directElements()
	existing := make(map[xml.Name]bool)
	for _, element := range properties.directElements() {
		existing[element.name] = true
	}
	for index := len(defaultElements) - 1; index >= 0; index-- {
		if existing[defaultElements[index].name] {
			continue
		}
		insertPackageXMLChildBeforeFirstElement(properties, defaultElements[index].clone())
		existing[defaultElements[index].name] = true
	}
	return nil
}

type mergedNumberingMaps struct {
	numbers   map[string]string
	abstracts map[string]string
	pictures  map[string]string
}

func buildMergedNumbering(packages []*mergeDOCXPackage, styleMaps []map[string]string, relationshipMaps []map[string]string) (*packageXMLDocument, []mergedNumberingMaps, string, error) {
	numberingMaps := make([]mergedNumberingMaps, len(packages))
	usedNumbers, usedAbstracts, usedPictures := make(map[string]bool), make(map[string]bool), make(map[string]bool)
	nextNumber, nextAbstract, nextPicture := 1, 0, 0
	var merged *packageXMLDocument
	pathName := ""
	baseIndex := -1

	for sourceIndex, input := range packages {
		mapping := mergedNumberingMaps{
			numbers:   make(map[string]string),
			abstracts: make(map[string]string),
			pictures:  make(map[string]string),
		}
		if input.numbering == nil {
			numberingMaps[sourceIndex] = mapping
			continue
		}
		if merged == nil {
			merged = &packageXMLDocument{
				prefix: append([]byte(nil), input.numbering.prefix...),
				root:   input.numbering.root.clone(),
				suffix: append([]byte(nil), input.numbering.suffix...),
			}
			pathName = input.numberingPath
			baseIndex = sourceIndex
		}
		if sourceIndex > 0 {
			if err := ensureMergeNamespace(merged.root, input.numbering.root); err != nil {
				return nil, nil, "", err
			}
		}
		elements := input.numbering.root.directElements()
		var err error
		if sourceIndex == baseIndex {
			mapping.abstracts, err = numberingDefinitionIdentityMap(elements, "abstractNum", "abstractNumId", usedAbstracts)
			if err != nil {
				return nil, nil, "", err
			}
			mapping.numbers, err = numberingDefinitionIdentityMap(elements, "num", "numId", usedNumbers)
			if err != nil {
				return nil, nil, "", err
			}
			mapping.pictures, err = numberingDefinitionIdentityMap(elements, "numPicBullet", "numPicBulletId", usedPictures)
			if err != nil {
				return nil, nil, "", err
			}
		} else {
			mapping.abstracts, err = allocateNumberingDefinitionMap(elements, "abstractNum", "abstractNumId", usedAbstracts, &nextAbstract)
			if err != nil {
				return nil, nil, "", err
			}
			mapping.numbers, err = allocateNumberingDefinitionMap(elements, "num", "numId", usedNumbers, &nextNumber)
			if err != nil {
				return nil, nil, "", err
			}
			mapping.pictures, err = allocateNumberingDefinitionMap(elements, "numPicBullet", "numPicBulletId", usedPictures, &nextPicture)
			if err != nil {
				return nil, nil, "", err
			}
		}
		numberingMaps[sourceIndex] = mapping

		if sourceIndex == baseIndex {
			if sourceIndex > 0 {
				for _, definition := range merged.root.directElements() {
					if !isWordElement(definition, "numPicBullet") && !isWordElement(definition, "abstractNum") && !isWordElement(definition, "num") {
						continue
					}
					if err := mergeWordStyleReferences(definition, styleMaps[sourceIndex]); err != nil {
						return nil, nil, "", err
					}
					if err := mergeWordNumberingReferences(definition, mapping.numbers, mapping.abstracts, mapping.pictures); err != nil {
						return nil, nil, "", err
					}
					if len(relationshipMaps) > sourceIndex {
						if err := remapOfficeRelationshipReferences(definition, relationshipMaps[sourceIndex]); err != nil {
							return nil, nil, "", err
						}
					}
				}
			}
			continue
		}
		for _, element := range elements {
			if !isWordElement(element, "numPicBullet") && !isWordElement(element, "abstractNum") && !isWordElement(element, "num") {
				continue
			}
			copy := element.clone()
			if err := mergeWordStyleReferences(copy, styleMaps[sourceIndex]); err != nil {
				return nil, nil, "", err
			}
			if err := mergeWordNumberingReferences(copy, mapping.numbers, mapping.abstracts, mapping.pictures); err != nil {
				return nil, nil, "", err
			}
			if len(relationshipMaps) > sourceIndex {
				if err := remapOfficeRelationshipReferences(copy, relationshipMaps[sourceIndex]); err != nil {
					return nil, nil, "", err
				}
			}
			insertMergedNumberingElement(merged.root, copy)
		}
	}
	return merged, numberingMaps, pathName, nil
}

func numberingDefinitionIdentityMap(elements []*packageXMLNode, elementName, idName string, used map[string]bool) (map[string]string, error) {
	mapping := make(map[string]string)
	seen := make(map[string]bool)
	for _, element := range elements {
		if !isWordElement(element, elementName) {
			continue
		}
		id, ok := wordAttrValue(element, idName)
		if !ok || id == "" || seen[id] || used[id] {
			return nil, fmt.Errorf("definição %s duplicada ou sem identificador '%s'", elementName, idName)
		}
		if err := validateNumberingDefinitionID(id, elementName, idName); err != nil {
			return nil, err
		}
		seen[id], used[id], mapping[id] = true, true, id
	}
	return mapping, nil
}

func allocateNumberingDefinitionMap(elements []*packageXMLNode, elementName, idName string, used map[string]bool, next *int) (map[string]string, error) {
	mapping := make(map[string]string)
	seen := make(map[string]bool)
	for _, element := range elements {
		if !isWordElement(element, elementName) {
			continue
		}
		id, ok := wordAttrValue(element, idName)
		if !ok || id == "" || seen[id] {
			return nil, fmt.Errorf("definição %s duplicada ou sem identificador '%s'", elementName, idName)
		}
		if err := validateNumberingDefinitionID(id, elementName, idName); err != nil {
			return nil, err
		}
		seen[id] = true
		for used[strconv.Itoa(*next)] {
			*next++
		}
		replacement := strconv.Itoa(*next)
		*next++
		used[replacement] = true
		mapping[id] = replacement
	}
	return mapping, nil
}

func validateNumberingDefinitionID(id, elementName, idName string) error {
	value, err := strconv.ParseInt(id, 10, 64)
	if err != nil || value < 0 {
		return fmt.Errorf("identificador '%s' inválido '%s' na definição %s", idName, id, elementName)
	}
	return nil
}

func insertMergedNumberingElement(root, element *packageXMLNode) {
	priority := map[string]int{"numPicBullet": 0, "abstractNum": 1, "num": 2}
	elementPriority := priority[element.name.Local]
	for i, part := range root.parts {
		if part.node == nil {
			continue
		}
		otherPriority, known := priority[part.node.name.Local]
		if known && otherPriority > elementPriority {
			root.parts = append(root.parts, packageXMLPart{})
			copy(root.parts[i+1:], root.parts[i:])
			root.parts[i] = packageXMLPart{node: element}
			return
		}
	}
	insertPackageXMLChildBeforeWordElement(root, element, "numIdMacAtCleanup")
}

func mergeDOCXPackages(packages []*mergeDOCXPackage) (map[string][]byte, error) {
	copier, err := newMergePartCopier(packages)
	if err != nil {
		return nil, err
	}
	styles, styleMaps, stylesPath, err := buildMergedStyles(packages)
	if err != nil {
		return nil, err
	}
	if styles != nil {
		if packages[0].styles == nil {
			stylesPath = uniqueMergePartName(copier.entries, "word/styles.xml")
		}
		if err := setOutputPartContentType(copier, packages, stylesPath, firstExistingStylesPath(packages), "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"); err != nil {
			return nil, err
		}
	}

	numbering, numberingMaps, numberingPath, err := buildMergedNumbering(packages, styleMaps, nil)
	if err != nil {
		return nil, err
	}
	copier.styleMaps = styleMaps
	copier.numberingMaps = numberingMaps
	if numbering != nil {
		if packages[0].numbering == nil {
			numberingPath = uniqueMergePartName(copier.entries, "word/numbering.xml")
		}
		if err := setOutputPartContentType(copier, packages, numberingPath, firstExistingNumberingPath(packages), "application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"); err != nil {
			return nil, err
		}
	}

	masterRelationships := append([]opcRelationship(nil), packages[0].documentRels...)
	if styles != nil {
		masterRelationships, err = ensureCoreDocumentRelationship(masterRelationships, "styles", stylesPath)
		if err != nil {
			return nil, err
		}
	}
	if numbering != nil {
		masterRelationships, err = ensureCoreDocumentRelationship(masterRelationships, "numbering", numberingPath)
		if err != nil {
			return nil, err
		}
	}
	mergedDocumentRelationships, documentRelationshipMaps, err := buildMergedDocumentRelationships(packages, copier, masterRelationships)
	if err != nil {
		return nil, err
	}

	numberingRelationshipMaps, numberingRelationships, err := buildMergedNumberingRelationshipMaps(packages, copier, numberingPath)
	if err != nil {
		return nil, err
	}
	if numbering != nil {
		numbering, numberingMaps, _, err = buildMergedNumbering(packages, styleMaps, numberingRelationshipMaps)
		if err != nil {
			return nil, err
		}
		copier.entries[numberingPath] = numbering.bytes()
		if len(numberingRelationships) > 0 {
			relsPath := relationshipPartName(numberingPath)
			relsBytes, err := marshalOPCRelationships(numberingRelationships)
			if err != nil {
				return nil, err
			}
			copier.entries[relsPath] = relsBytes
			if _, exists := copier.contentTypes[relsPath]; !exists {
				copier.contentTypes[relsPath] = "application/vnd.openxmlformats-package.relationships+xml"
			}
		}
	}
	if err := remapMergedStylesNumberingReferences(styles, packages, styleMaps, numberingMaps); err != nil {
		return nil, err
	}

	mergedDocument, err := buildMergedMainDocument(packages, documentRelationshipMaps, styleMaps, numberingMaps)
	if err != nil {
		return nil, err
	}
	copier.entries[mergeDocumentPath] = mergedDocument.bytes()
	if _, exists := copier.contentTypes[mergeDocumentPath]; !exists {
		copier.contentTypes[mergeDocumentPath] = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
	}
	documentRelsPath := relationshipPartName(mergeDocumentPath)
	serializedRels, err := marshalOPCRelationships(mergedDocumentRelationships)
	if err != nil {
		return nil, err
	}
	copier.entries[documentRelsPath] = serializedRels
	if _, exists := copier.contentTypes[documentRelsPath]; !exists {
		copier.contentTypes[documentRelsPath] = "application/vnd.openxmlformats-package.relationships+xml"
	}
	if styles != nil {
		copier.entries[stylesPath] = styles.bytes()
	}
	if numbering != nil {
		copier.entries[numberingPath] = numbering.bytes()
	}
	contentTypesBytes, err := marshalMergeContentTypes(copier.contentTypes)
	if err != nil {
		return nil, err
	}
	copier.entries[mergeContentTypesPath] = contentTypesBytes
	return copier.entries, nil
}

func setOutputPartContentType(copier *mergePartCopier, packages []*mergeDOCXPackage, outputPart, sourcePart, fallback string) error {
	if _, exists := copier.contentTypes[outputPart]; exists {
		return nil
	}
	for _, input := range packages {
		if sourcePart == "" || sourcePart != input.stylesPath && sourcePart != input.numberingPath {
			continue
		}
		contentType, err := packagePartContentType(input.contentTypes, sourcePart)
		if err == nil {
			copier.contentTypes[outputPart] = contentType
			return nil
		}
	}
	copier.contentTypes[outputPart] = fallback
	return nil
}

func firstExistingStylesPath(packages []*mergeDOCXPackage) string {
	for _, input := range packages {
		if input.styles != nil {
			return input.stylesPath
		}
	}
	return ""
}

func firstExistingNumberingPath(packages []*mergeDOCXPackage) string {
	for _, input := range packages {
		if input.numbering != nil {
			return input.numberingPath
		}
	}
	return ""
}

func ensureCoreDocumentRelationship(relationships []opcRelationship, relationType, targetPart string) ([]opcRelationship, error) {
	for _, relationship := range relationships {
		if relationshipTypeSuffix(relationship.Type) != relationType {
			continue
		}
		part, _, err := resolveOPCTarget(mergeDocumentPath, relationship.Target)
		if err != nil {
			return nil, err
		}
		if part != targetPart {
			return nil, fmt.Errorf("relacionamento '%s' aponta para '%s', esperado '%s'", relationType, part, targetPart)
		}
		return relationships, nil
	}
	used := make(map[string]bool)
	for _, relationship := range relationships {
		used[relationship.ID] = true
	}
	id := "rIdCaramelCore" + strings.ToUpper(relationType[:1]) + relationType[1:]
	for suffix := 2; used[id]; suffix++ {
		id = fmt.Sprintf("rIdCaramelCore%s%d", strings.ToUpper(relationType[:1])+relationType[1:], suffix)
	}
	return append(relationships, opcRelationship{
		ID:     id,
		Type:   officeRelationshipsNS + "/" + relationType,
		Target: relativeOPCTarget(mergeDocumentPath, targetPart),
	}), nil
}

func buildMergedNumberingRelationshipMaps(packages []*mergeDOCXPackage, copier *mergePartCopier, outputNumberingPath string) ([]map[string]string, []opcRelationship, error) {
	mappings := make([]map[string]string, len(packages))
	var merged []opcRelationship
	used := make(map[string]bool)
	nextID := 1
	for sourceIndex, input := range packages {
		mapping := make(map[string]string)
		if input.numbering == nil {
			mappings[sourceIndex] = mapping
			continue
		}
		relsPath := relationshipPartName(input.numberingPath)
		if _, exists := input.files[relsPath]; !exists {
			mappings[sourceIndex] = mapping
			continue
		}
		data, err := input.readPart(relsPath)
		if err != nil {
			return nil, nil, err
		}
		var relationships []opcRelationship
		if err := unmarshalOPCRelationships(data, &relationships); err != nil {
			return nil, nil, fmt.Errorf("relacionamentos de numeração inválidos: %w", err)
		}
		if err := validateRelationshipIDs(relationships); err != nil {
			return nil, nil, err
		}
		for _, relationship := range relationships {
			if sourceIndex == 0 {
				mapping[relationship.ID] = relationship.ID
				used[relationship.ID] = true
				merged = append(merged, relationship)
				continue
			}
			newID := ""
			for newID == "" || used[newID] {
				newID = fmt.Sprintf("rIdCaramelNumbering%d", nextID)
				nextID++
			}
			used[newID] = true
			mapping[relationship.ID] = newID
			outputRelationship := relationship
			outputRelationship.ID = newID
			if relationship.TargetMode != "External" {
				targetPart, suffix, err := resolveOPCTarget(input.numberingPath, relationship.Target)
				if err != nil {
					return nil, nil, err
				}
				outputTarget, err := copier.copyRelatedPart(sourceIndex, targetPart)
				if err != nil {
					return nil, nil, err
				}
				outputRelationship.Target = relativeOPCTarget(outputNumberingPath, outputTarget) + suffix
			}
			merged = append(merged, outputRelationship)
		}
		mappings[sourceIndex] = mapping
	}
	return mappings, merged, nil
}

func buildMergedMainDocument(packages []*mergeDOCXPackage, relationshipMaps, styleMaps []map[string]string, numberingMaps []mergedNumberingMaps) (*packageXMLDocument, error) {
	merged := &packageXMLDocument{
		prefix: append([]byte(nil), packages[0].document.prefix...),
		root:   packages[0].document.root.clone(),
		suffix: append([]byte(nil), packages[0].document.suffix...),
	}
	outBody := merged.root.firstElement(wordXMLNamespace, "body")
	if outBody == nil {
		return nil, fmt.Errorf("o documento principal não contém w:body")
	}
	var bodyElements []*packageXMLNode
	for sourceIndex, input := range packages {
		if err := ensureMergeNamespace(merged.root, input.document.root); err != nil {
			return nil, err
		}
		sourceBody := input.document.root.firstElement(wordXMLNamespace, "body")
		elements := sourceBody.directElements()
		var finalSection *packageXMLNode
		if sourceIndex < len(packages)-1 {
			elements, finalSection = splitFinalSectionForMerge(elements)
		}
		for _, element := range elements {
			copy := element.clone()
			if sourceIndex > 0 {
				defaults := defaultWordStyles(input.styles, styleMaps[sourceIndex])
				if err := addDefaultStyleReferences(copy, defaults); err != nil {
					return nil, err
				}
			}
			if err := mergeWordStyleReferences(copy, styleMaps[sourceIndex]); err != nil {
				return nil, err
			}
			if len(numberingMaps) > sourceIndex {
				mapping := numberingMaps[sourceIndex]
				if err := mergeWordNumberingReferences(copy, mapping.numbers, mapping.abstracts, mapping.pictures); err != nil {
					return nil, err
				}
			}
			if len(relationshipMaps) > sourceIndex {
				if err := remapOfficeRelationshipReferences(copy, relationshipMaps[sourceIndex]); err != nil {
					return nil, fmt.Errorf("referência do documento %d inválida: %w", sourceIndex+1, err)
				}
			}
			bodyElements = append(bodyElements, copy)
		}
		if sourceIndex < len(packages)-1 {
			if finalSection != nil {
				section := finalSection.clone()
				if err := mergeWordStyleReferences(section, styleMaps[sourceIndex]); err != nil {
					return nil, err
				}
				if len(numberingMaps) > sourceIndex {
					mapping := numberingMaps[sourceIndex]
					if err := mergeWordNumberingReferences(section, mapping.numbers, mapping.abstracts, mapping.pictures); err != nil {
						return nil, err
					}
				}
				if len(relationshipMaps) > sourceIndex {
					if err := remapOfficeRelationshipReferences(section, relationshipMaps[sourceIndex]); err != nil {
						return nil, fmt.Errorf("referência de seção do documento %d inválida: %w", sourceIndex+1, err)
					}
				}
				boundary, err := newSectionBoundaryParagraph(section)
				if err != nil {
					return nil, err
				}
				bodyElements = append(bodyElements, boundary)
			} else {
				pageBreak, err := newPageBreakParagraph()
				if err != nil {
					return nil, err
				}
				bodyElements = append(bodyElements, pageBreak)
			}
		}
	}
	outBody.replaceElements(bodyElements)
	return merged, nil
}

func splitFinalSectionForMerge(elements []*packageXMLNode) ([]*packageXMLNode, *packageXMLNode) {
	if len(elements) == 0 {
		return elements, nil
	}
	last := elements[len(elements)-1]
	if isWordElement(last, "sectPr") {
		return elements[:len(elements)-1], last
	}
	if !isWordElement(last, "p") {
		return elements, nil
	}
	section := paragraphSectionProperties(last)
	if section == nil {
		return elements, nil
	}
	copy := last.clone()
	removeParagraphSectionProperties(copy)
	result := append([]*packageXMLNode(nil), elements...)
	result[len(result)-1] = copy
	return result, section
}

func newPageBreakParagraph() (*packageXMLNode, error) {
	node, err := parsePackageXMLNode(`<w:p xmlns:w="` + wordXMLNamespace + `"><w:r><w:br w:type="page"/></w:r></w:p>`)
	if err != nil {
		return nil, err
	}
	return node, nil
}

func newSectionBoundaryParagraph(section *packageXMLNode) (*packageXMLNode, error) {
	if typeNode := section.firstElement(wordXMLNamespace, "type"); typeNode != nil {
		if err := setWordAttr(typeNode, "val", "nextPage"); err != nil {
			return nil, err
		}
	} else {
		typeNode, err := parsePackageXMLNode(`<w:type xmlns:w="` + wordXMLNamespace + `" w:val="nextPage"/>`)
		if err != nil {
			return nil, err
		}
		insertPackageXMLChildBeforeFirstElement(section, typeNode)
	}
	p := &packageXMLNode{
		name:  xml.Name{Space: wordXMLNamespace, Local: "p"},
		open:  []byte(`<w:p xmlns:w="` + wordXMLNamespace + `">`),
		close: []byte(`</w:p>`),
	}
	pPr := &packageXMLNode{
		name: xml.Name{Space: wordXMLNamespace, Local: "pPr"},
		open: []byte(`<w:pPr>`), close: []byte(`</w:pPr>`),
		parts: []packageXMLPart{{node: section}},
	}
	p.parts = []packageXMLPart{{node: pPr}}
	return p, nil
}

func writeMergedDOCXAtomic(entries map[string][]byte, outputPath string) error {
	return writeMergedDOCXAtomicImpl(entries, outputPath)
}

func copyArchiveEntry(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	return content, nil
}

func ensureMergeNamespace(destination, source *packageXMLNode) error {
	const xmlnsNamespace = "xmlns"
	declared := make(map[string]string)
	for _, attribute := range destination.attrs {
		if attribute.Name.Space == xmlnsNamespace {
			declared[attribute.Name.Local] = attribute.Value
		}
		if attribute.Name.Space == "" && attribute.Name.Local == "xmlns" {
			declared[""] = attribute.Value
		}
	}
	for _, attribute := range source.attrs {
		prefix := ""
		isDeclaration := false
		if attribute.Name.Space == xmlnsNamespace {
			prefix, isDeclaration = attribute.Name.Local, true
		} else if attribute.Name.Space == "" && attribute.Name.Local == "xmlns" {
			isDeclaration = true
		}
		if !isDeclaration {
			continue
		}
		if current, exists := declared[prefix]; exists {
			if current != attribute.Value {
				return fmt.Errorf("prefixo XML '%s' é usado com namespaces diferentes entre documentos", prefix)
			}
			continue
		}
		qualified := "xmlns"
		if prefix != "" {
			qualified += ":" + prefix
		}
		if err := destination.setAttribute(xmlnsNamespace, prefix, qualified, attribute.Value); err != nil {
			return err
		}
		declared[prefix] = attribute.Value
	}

	for index, attribute := range source.attrs {
		if attribute.Name.Space != markupCompatibilityNS {
			continue
		}
		if attribute.Name.Local != "Ignorable" && attribute.Name.Local != "MustUnderstand" && attribute.Name.Local != "ProcessContent" && attribute.Name.Local != "PreserveElements" && attribute.Name.Local != "PreserveAttributes" {
			continue
		}
		current, _ := destination.attributeValue(markupCompatibilityNS, attribute.Name.Local)
		merged := uniqueSpaceSeparated(current, attribute.Value)
		qualified := source.attrNames[index]
		if err := destination.setAttribute(markupCompatibilityNS, attribute.Name.Local, qualified, merged); err != nil {
			return err
		}
	}
	return nil
}

func uniqueSpaceSeparated(left, right string) string {
	seen := make(map[string]bool)
	var values []string
	for _, value := range strings.Fields(left + " " + right) {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return strings.Join(values, " ")
}

func mergeWordStyleReferences(node *packageXMLNode, mapping map[string]string) error {
	styleReferences := map[string]bool{
		"pStyle": true, "rStyle": true, "tblStyle": true, "basedOn": true,
		"next": true, "link": true, "styleLink": true, "numStyleLink": true,
	}
	var walk func(*packageXMLNode) error
	walk = func(current *packageXMLNode) error {
		if current.name.Space == wordXMLNamespace && styleReferences[current.name.Local] {
			if value, ok := wordAttrValue(current, "val"); ok {
				if replacement, exists := mapping[value]; exists && replacement != value {
					if err := setWordAttr(current, "val", replacement); err != nil {
						return err
					}
				}
			}
		}
		for _, part := range current.parts {
			if part.node != nil {
				if err := walk(part.node); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(node)
}

func mergeWordNumberingReferences(node *packageXMLNode, numbering, abstract, picture map[string]string) error {
	var walk func(*packageXMLNode) error
	walk = func(current *packageXMLNode) error {
		if current.name.Space == wordXMLNamespace {
			var mapping map[string]string
			attribute := "val"
			switch current.name.Local {
			case "numId":
				mapping = numbering
			case "abstractNumId":
				mapping = abstract
			case "lvlPicBulletId":
				mapping = picture
			}
			if mapping != nil {
				if value, ok := wordAttrValue(current, attribute); ok {
					if replacement, exists := mapping[value]; exists {
						if replacement != value {
							if err := setWordAttr(current, attribute, replacement); err != nil {
								return err
							}
						}
					} else if current.name.Local != "numId" || value != "0" {
						return fmt.Errorf("referência de numeração '%s' não tem definição no documento", value)
					}
				}
			}
			if current.name.Local == "num" || current.name.Local == "abstractNum" || current.name.Local == "numPicBullet" {
				idAttribute := map[string]string{"num": "numId", "abstractNum": "abstractNumId", "numPicBullet": "numPicBulletId"}[current.name.Local]
				idMapping := map[string]map[string]string{"num": numbering, "abstractNum": abstract, "numPicBullet": picture}[current.name.Local]
				if value, ok := wordAttrValue(current, idAttribute); ok {
					if replacement, exists := idMapping[value]; exists && replacement != value {
						if err := setWordAttr(current, idAttribute, replacement); err != nil {
							return err
						}
					}
				}
			}
		}
		for _, part := range current.parts {
			if part.node != nil {
				if err := walk(part.node); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(node)
}

func remapOfficeRelationshipReferences(node *packageXMLNode, mapping map[string]string) error {
	var walk func(*packageXMLNode) error
	walk = func(current *packageXMLNode) error {
		for index, attribute := range current.attrs {
			if attribute.Name.Space != officeRelationshipsNS || (attribute.Name.Local != "id" && attribute.Name.Local != "embed" && attribute.Name.Local != "link") {
				continue
			}
			replacement, exists := mapping[attribute.Value]
			if !exists {
				return fmt.Errorf("referência OOXML '%s' não tem relacionamento correspondente", attribute.Value)
			}
			if err := current.setAttribute(officeRelationshipsNS, attribute.Name.Local, current.attrNames[index], replacement); err != nil {
				return err
			}
		}
		for _, part := range current.parts {
			if part.node != nil {
				if err := walk(part.node); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(node)
}

func defaultWordStyles(styles *packageXMLDocument, styleMapping map[string]string) map[string]string {
	defaults := make(map[string]string)
	if styles == nil {
		return defaults
	}
	for _, node := range styles.root.directElements() {
		if !isWordElement(node, "style") {
			continue
		}
		defaultValue, _ := wordAttrValue(node, "default")
		if defaultValue != "1" && !strings.EqualFold(defaultValue, "true") && !strings.EqualFold(defaultValue, "on") {
			continue
		}
		styleType, _ := wordAttrValue(node, "type")
		styleID, _ := wordAttrValue(node, "styleId")
		if mapped, exists := styleMapping[styleID]; exists && styleID != "" {
			defaults[styleType] = mapped
		}
	}
	return defaults
}

func addDefaultStyleReferences(node *packageXMLNode, defaults map[string]string) error {
	var walk func(*packageXMLNode) error
	walk = func(current *packageXMLNode) error {
		if current.name.Space == wordXMLNamespace {
			switch current.name.Local {
			case "p":
				if style := defaults["paragraph"]; style != "" {
					pPr := current.firstElement(wordXMLNamespace, "pPr")
					if pPr == nil || pPr.firstElement(wordXMLNamespace, "pStyle") == nil {
						property, err := parsePackageXMLNode(`<w:pStyle xmlns:w="` + wordXMLNamespace + `" w:val="` + xmlAttributeLiteral(style) + `"/>`)
						if err != nil {
							return err
						}
						if pPr == nil {
							pPr, err = parsePackageXMLNode(`<w:pPr xmlns:w="` + wordXMLNamespace + `"/>`)
							if err != nil {
								return err
							}
							insertPackageXMLChildBeforeFirstElement(current, pPr)
						}
						insertPackageXMLChildBeforeFirstElement(pPr, property)
					}
				}
			case "r":
				if style := defaults["character"]; style != "" {
					rPr := current.firstElement(wordXMLNamespace, "rPr")
					if rPr == nil || rPr.firstElement(wordXMLNamespace, "rStyle") == nil {
						property, err := parsePackageXMLNode(`<w:rStyle xmlns:w="` + wordXMLNamespace + `" w:val="` + xmlAttributeLiteral(style) + `"/>`)
						if err != nil {
							return err
						}
						if rPr == nil {
							rPr, err = parsePackageXMLNode(`<w:rPr xmlns:w="` + wordXMLNamespace + `"/>`)
							if err != nil {
								return err
							}
							insertPackageXMLChildBeforeFirstElement(current, rPr)
						}
						insertPackageXMLChildBeforeFirstElement(rPr, property)
					}
				}
			case "tbl":
				if style := defaults["table"]; style != "" {
					tblPr := current.firstElement(wordXMLNamespace, "tblPr")
					if tblPr == nil || tblPr.firstElement(wordXMLNamespace, "tblStyle") == nil {
						property, err := parsePackageXMLNode(`<w:tblStyle xmlns:w="` + wordXMLNamespace + `" w:val="` + xmlAttributeLiteral(style) + `"/>`)
						if err != nil {
							return err
						}
						if tblPr == nil {
							tblPr, err = parsePackageXMLNode(`<w:tblPr xmlns:w="` + wordXMLNamespace + `"/>`)
							if err != nil {
								return err
							}
							insertPackageXMLChildBeforeFirstElement(current, tblPr)
						}
						insertPackageXMLChildBeforeFirstElement(tblPr, property)
					}
				}
			}
		}
		for _, part := range current.parts {
			if part.node != nil {
				if err := walk(part.node); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(node)
}

func xmlAttributeLiteral(value string) string {
	encoded, _ := escapeXMLAttributeValue(value)
	return string(encoded)
}
