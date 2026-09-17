package docx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// packageXMLNode keeps the original markup for every element. DOCX files rely
// on namespace prefixes in values such as mc:Ignorable, so rewriting the whole
// document through xml.Encoder would change more than the requested content.
type packageXMLNode struct {
	name          xml.Name
	qualifiedName string
	attrs         []xml.Attr
	attrNames     []string
	open          []byte
	close         []byte
	parts         []packageXMLPart
	lastByte      int
	emptyTag      bool
}

type packageXMLPart struct {
	raw  []byte
	node *packageXMLNode
}

type packageXMLDocument struct {
	prefix []byte
	root   *packageXMLNode
	suffix []byte
}

func parsePackageXML(source []byte) (*packageXMLDocument, error) {
	decoder := xml.NewDecoder(bytes.NewReader(source))
	var stack []*packageXMLNode
	document := &packageXMLDocument{}
	rootEnd := -1

	for {
		before := int(decoder.InputOffset())
		token, err := decoder.Token()
		after := int(decoder.InputOffset())
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("XML inválido: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			if len(stack) == 0 {
				if document.root != nil {
					return nil, fmt.Errorf("XML contém mais de um elemento raiz")
				}
				document.prefix = append([]byte(nil), source[:before]...)
			} else {
				parent := stack[len(stack)-1]
				parent.appendRaw(source[parent.lastByte:before])
			}

			open := append([]byte(nil), source[before:after]...)
			attrNames := rawXMLAttributeNames(open)
			if len(attrNames) != len(value.Attr) {
				return nil, fmt.Errorf("XML contém atributos que não puderam ser preservados (%d nomes, %d atributos)", len(attrNames), len(value.Attr))
			}
			node := &packageXMLNode{
				name:          value.Name,
				qualifiedName: rawXMLElementName(open),
				attrs:         append([]xml.Attr(nil), value.Attr...),
				attrNames:     attrNames,
				open:          open,
				lastByte:      after,
				emptyTag:      bytes.HasSuffix(bytes.TrimSpace(open), []byte("/>")),
			}
			if len(stack) == 0 {
				document.root = node
			} else {
				parent := stack[len(stack)-1]
				parent.parts = append(parent.parts, packageXMLPart{node: node})
			}
			stack = append(stack, node)

		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("XML contém um fechamento sem abertura")
			}
			node := stack[len(stack)-1]
			if node.name != value.Name {
				return nil, fmt.Errorf("XML contém elementos aninhados incorretamente")
			}
			if !node.emptyTag {
				node.appendRaw(source[node.lastByte:before])
				node.close = append([]byte(nil), source[before:after]...)
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				rootEnd = after
			} else {
				stack[len(stack)-1].lastByte = after
			}
		}
	}

	if len(stack) != 0 {
		return nil, fmt.Errorf("XML terminou antes do fechamento de todos os elementos")
	}
	if document.root == nil || rootEnd < 0 {
		return nil, fmt.Errorf("XML não contém um elemento raiz")
	}
	document.suffix = append([]byte(nil), source[rootEnd:]...)
	return document, nil
}

func (n *packageXMLNode) appendRaw(raw []byte) {
	if len(raw) == 0 {
		return
	}
	n.parts = append(n.parts, packageXMLPart{raw: append([]byte(nil), raw...)})
}

func (n *packageXMLNode) appendTo(buffer *bytes.Buffer) {
	buffer.Write(n.open)
	for _, part := range n.parts {
		if part.node != nil {
			part.node.appendTo(buffer)
		} else {
			buffer.Write(part.raw)
		}
	}
	buffer.Write(n.close)
}

func (d *packageXMLDocument) bytes() []byte {
	var buffer bytes.Buffer
	buffer.Grow(len(d.prefix) + len(d.suffix) + 4096)
	buffer.Write(d.prefix)
	d.root.appendTo(&buffer)
	buffer.Write(d.suffix)
	return buffer.Bytes()
}

func (n *packageXMLNode) clone() *packageXMLNode {
	clone := &packageXMLNode{
		name:          n.name,
		qualifiedName: n.qualifiedName,
		attrs:         append([]xml.Attr(nil), n.attrs...),
		attrNames:     append([]string(nil), n.attrNames...),
		open:          append([]byte(nil), n.open...),
		close:         append([]byte(nil), n.close...),
		emptyTag:      n.emptyTag,
	}
	for _, part := range n.parts {
		if part.node != nil {
			clone.parts = append(clone.parts, packageXMLPart{node: part.node.clone()})
		} else {
			clone.parts = append(clone.parts, packageXMLPart{raw: append([]byte(nil), part.raw...)})
		}
	}
	return clone
}

func (n *packageXMLNode) directElements() []*packageXMLNode {
	var elements []*packageXMLNode
	for _, part := range n.parts {
		if part.node != nil {
			elements = append(elements, part.node)
		}
	}
	return elements
}

func (n *packageXMLNode) firstElement(namespace, local string) *packageXMLNode {
	for _, part := range n.parts {
		if part.node != nil && part.node.name.Space == namespace && part.node.name.Local == local {
			return part.node
		}
	}
	return nil
}

func (n *packageXMLNode) replaceElements(elements []*packageXMLNode) {
	n.parts = n.parts[:0]
	for i, element := range elements {
		if i > 0 {
			n.parts = append(n.parts, packageXMLPart{raw: []byte("\n    ")})
		}
		n.parts = append(n.parts, packageXMLPart{node: element})
	}
	if len(elements) > 0 {
		n.parts = append(n.parts, packageXMLPart{raw: []byte("\n  ")})
	}
}

func rawXMLAttributeNames(open []byte) []string {
	spans := rawXMLAttributeSpans(open)
	names := make([]string, len(spans))
	for i, span := range spans {
		names[i] = string(open[span.nameStart:span.nameEnd])
	}
	return names
}

func rawXMLElementName(open []byte) string {
	index := 1
	for index < len(open) && !isXMLSpace(open[index]) && open[index] != '/' && open[index] != '>' {
		index++
	}
	return string(open[1:index])
}

type rawXMLAttributeSpan struct {
	nameStart  int
	nameEnd    int
	valueStart int
	valueEnd   int
}

func rawXMLAttributeSpans(open []byte) []rawXMLAttributeSpan {
	var spans []rawXMLAttributeSpan
	index := 1
	for index < len(open) && !isXMLSpace(open[index]) && open[index] != '/' && open[index] != '>' {
		index++
	}
	for index < len(open) {
		for index < len(open) && isXMLSpace(open[index]) {
			index++
		}
		if index >= len(open) || open[index] == '>' || open[index] == '/' {
			break
		}
		nameStart := index
		for index < len(open) && !isXMLSpace(open[index]) && open[index] != '=' && open[index] != '/' && open[index] != '>' {
			index++
		}
		nameEnd := index
		for index < len(open) && isXMLSpace(open[index]) {
			index++
		}
		if index >= len(open) || open[index] != '=' {
			return spans
		}
		index++
		for index < len(open) && isXMLSpace(open[index]) {
			index++
		}
		if index >= len(open) || (open[index] != '\'' && open[index] != '"') {
			return spans
		}
		quote := open[index]
		index++
		valueStart := index
		for index < len(open) && open[index] != quote {
			index++
		}
		if index >= len(open) {
			return spans
		}
		spans = append(spans, rawXMLAttributeSpan{
			nameStart:  nameStart,
			nameEnd:    nameEnd,
			valueStart: valueStart,
			valueEnd:   index,
		})
		index++
	}
	return spans
}

func isXMLSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func (n *packageXMLNode) attributeIndex(namespace, local string) int {
	for i, attr := range n.attrs {
		if attr.Name.Space == namespace && attr.Name.Local == local {
			return i
		}
	}
	return -1
}

func (n *packageXMLNode) attributeValue(namespace, local string) (string, bool) {
	index := n.attributeIndex(namespace, local)
	if index < 0 {
		return "", false
	}
	return n.attrs[index].Value, true
}

func (n *packageXMLNode) setAttribute(namespace, local, qualifiedName, value string) error {
	index := n.attributeIndex(namespace, local)
	if index >= 0 {
		spans := rawXMLAttributeSpans(n.open)
		if index >= len(spans) || n.attrNames[index] != string(n.open[spans[index].nameStart:spans[index].nameEnd]) {
			return fmt.Errorf("não foi possível localizar o atributo XML '%s'", n.attrNames[index])
		}
		encoded, err := escapeXMLAttributeValue(value)
		if err != nil {
			return err
		}
		span := spans[index]
		open := make([]byte, 0, len(n.open)+len(encoded)-(span.valueEnd-span.valueStart))
		open = append(open, n.open[:span.valueStart]...)
		open = append(open, encoded...)
		open = append(open, n.open[span.valueEnd:]...)
		n.open = open
		n.attrs[index].Value = value
		return nil
	}

	insertAt := len(n.open) - 1
	if bytes.HasSuffix(bytes.TrimSpace(n.open), []byte("/>")) {
		insertAt = bytes.LastIndex(n.open, []byte("/>"))
	}
	encoded, err := escapeXMLAttributeValue(value)
	if err != nil {
		return err
	}
	addition := make([]byte, 0, len(qualifiedName)+len(encoded)+4)
	addition = append(addition, ' ')
	addition = append(addition, qualifiedName...)
	addition = append(addition, '=', '"')
	addition = append(addition, encoded...)
	addition = append(addition, '"')
	open := make([]byte, 0, len(n.open)+len(addition))
	open = append(open, n.open[:insertAt]...)
	open = append(open, addition...)
	open = append(open, n.open[insertAt:]...)
	n.open = open
	n.attrs = append(n.attrs, xml.Attr{Name: xml.Name{Space: namespace, Local: local}, Value: value})
	n.attrNames = append(n.attrNames, qualifiedName)
	return nil
}

func escapeXMLAttributeValue(value string) ([]byte, error) {
	var buffer bytes.Buffer
	if err := xml.EscapeText(&buffer, []byte(value)); err != nil {
		return nil, err
	}
	escaped := strings.NewReplacer(`"`, "&#34;", "\t", "&#x9;", "\n", "&#xA;", "\r", "&#xD;").Replace(buffer.String())
	return []byte(escaped), nil
}

func appendPackageXMLChild(parent, child *packageXMLNode) {
	ensurePackageXMLNodeHasChildren(parent)
	if len(parent.parts) > 0 {
		last := parent.parts[len(parent.parts)-1]
		if last.node != nil || !bytes.HasSuffix(last.raw, []byte("\n")) {
			parent.parts = append(parent.parts, packageXMLPart{raw: []byte("\n")})
		}
	}
	parent.parts = append(parent.parts, packageXMLPart{node: child})
}

func insertPackageXMLChildBeforeFirstElement(parent, child *packageXMLNode) {
	ensurePackageXMLNodeHasChildren(parent)
	for i, part := range parent.parts {
		if part.node == nil {
			continue
		}
		parent.parts = append(parent.parts, packageXMLPart{})
		copy(parent.parts[i+1:], parent.parts[i:])
		parent.parts[i] = packageXMLPart{node: child}
		return
	}
	appendPackageXMLChild(parent, child)
}

func insertPackageXMLChildBeforeWordElement(parent, child *packageXMLNode, localNames ...string) {
	ensurePackageXMLNodeHasChildren(parent)
	targets := make(map[string]bool, len(localNames))
	for _, name := range localNames {
		targets[name] = true
	}
	for index, part := range parent.parts {
		if part.node == nil || part.node.name.Space != wordXMLNamespace || !targets[part.node.name.Local] {
			continue
		}
		parent.parts = append(parent.parts, packageXMLPart{})
		copy(parent.parts[index+1:], parent.parts[index:])
		parent.parts[index] = packageXMLPart{node: child}
		return
	}
	appendPackageXMLChild(parent, child)
}

func ensurePackageXMLNodeHasChildren(node *packageXMLNode) {
	if !node.emptyTag {
		return
	}
	closing := bytes.LastIndex(node.open, []byte("/>"))
	if closing < 0 {
		return
	}
	open := make([]byte, 0, len(node.open))
	open = append(open, node.open[:closing]...)
	open = append(open, '>')
	node.open = open
	node.close = []byte("</" + node.qualifiedName + ">")
	node.emptyTag = false
}

func parsePackageXMLNode(fragment string) (*packageXMLNode, error) {
	document, err := parsePackageXML([]byte(fragment))
	if err != nil {
		return nil, err
	}
	return document.root, nil
}

func wordAttrValue(node *packageXMLNode, local string) (string, bool) {
	return node.attributeValue(wordXMLNamespace, local)
}

func setWordAttr(node *packageXMLNode, local, value string) error {
	return node.setAttribute(wordXMLNamespace, local, "w:"+local, value)
}
