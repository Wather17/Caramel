package docx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

// packageXMLNode keeps the original markup for every element. DOCX files rely
// on namespace prefixes in values such as mc:Ignorable, so rewriting the whole
// document through xml.Encoder would change more than the requested content.
type packageXMLNode struct {
	name     xml.Name
	attrs    []xml.Attr
	open     []byte
	close    []byte
	parts    []packageXMLPart
	lastByte int
	emptyTag bool
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
			node := &packageXMLNode{
				name:     value.Name,
				attrs:    append([]xml.Attr(nil), value.Attr...),
				open:     open,
				lastByte: after,
				emptyTag: bytes.HasSuffix(bytes.TrimSpace(open), []byte("/>")),
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
		name:     n.name,
		attrs:    append([]xml.Attr(nil), n.attrs...),
		open:     append([]byte(nil), n.open...),
		close:    append([]byte(nil), n.close...),
		emptyTag: n.emptyTag,
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
