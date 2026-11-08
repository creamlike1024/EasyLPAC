package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"
)

// jnode is a node of a JSON document, in the document's order.
type jnode struct {
	key      string
	path     string
	value    string // scalars, as JSON text without quotes for strings
	kind     byte   // 'o' object, 'a' array, 's' scalar
	children []*jnode
}

// parseJSONTree parses a JSON value into nodes, keeping the key order.
func parseJSONTree(data []byte) (*jnode, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return parseValue(dec, "", "$")
}

func parseValue(dec *json.Decoder, key, path string) (*jnode, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	n := &jnode{key: key, path: path, kind: 's'}
	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			n.kind = 'o'
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k := kt.(string)
				child, err := parseValue(dec, k, path+"."+k)
				if err != nil {
					return nil, err
				}
				n.children = append(n.children, child)
			}
		case '[':
			n.kind = 'a'
			for i := 0; dec.More(); i++ {
				child, err := parseValue(dec, strconv.Itoa(i), fmt.Sprintf("%s[%d]", path, i))
				if err != nil {
					return nil, err
				}
				n.children = append(n.children, child)
			}
		}
		if _, err := dec.Token(); err != nil { // the closing delimiter
			return nil, err
		}
	case string:
		n.value = v
	case nil:
		n.value = "null"
	default:
		n.value = fmt.Sprint(v)
	}
	return n, nil
}

// summary describes a closed object or array.
func (n *jnode) summary() string {
	switch n.kind {
	case 'o':
		return fmt.Sprintf("{%d}", len(n.children))
	case 'a':
		return fmt.Sprintf("[%d]", len(n.children))
	}
	return n.value
}

// jsonTree shows a JSON tree whose objects and arrays open and close;
// note adds a muted remark after a node's value, and action, when set,
// builds controls at the end of a node's row.
func (g *gui) jsonTree(c *ui.Context, root *jnode, open map[string]bool, note func(n *jnode) string, action func(n *jnode)) {
	t := c.Theme()
	tree := ui.TreeBase(c)
	var node func(n *jnode)
	node = func(n *jnode) {
		c.Key(n.path)
		var openp *bool
		if n.kind != 's' {
			o := open[n.path]
			openp = &o
		}
		item := tree.Item(openp)
		row := item.Row.Gap(6).Padding(4, 8).PaddingStart(8 + 18*float32(item.Depth())).Radius(6)
		if row.Hovered() {
			row.Background(t.SurfaceHover)
		}
		if openp != nil && row.Clicked() {
			open[n.path] = !open[n.path]
		}
		row.Children(func() {
			if openp != nil {
				arrow := ui.Icon(c, iconChevron).Size(14, 14).TextColor(t.TextMuted)
				if open[n.path] {
					arrow.Rotate(90)
				}
			} else {
				ui.Box(c).Size(14, 14)
			}
			key := n.key
			if n.kind == 's' && len(n.path) > 0 && n.path[len(n.path)-1] == ']' {
				key = "[" + n.key + "]"
			}
			ui.Text(c, key).FontWeight(500).Shrink(0)
			if n.kind != 's' && open[n.path] {
				return
			}
			val := ui.Text(c, n.summary()).Font("monospace").FontSize(12).Selectable().MinWidth(0)
			if n.kind == 's' {
				val.TextColor(t.Text)
			} else {
				val.TextColor(t.TextMuted)
			}
			if note != nil {
				if s := note(n); s != "" {
					ui.Text(c, s).FontSize(12).TextColor(t.TextMuted).SingleLine().MinWidth(0)
				}
			}
			if action != nil {
				action(n)
			}
		})
		if openp != nil {
			item.Group(func() {
				for _, kid := range n.children {
					node(kid)
				}
			})
		}
	}
	tree.Root.Children(func() {
		for _, n := range root.children {
			node(n)
		}
	})
}

// setTreeOpen opens or closes every object and array under root.
func setTreeOpen(root *jnode, open map[string]bool, on bool) {
	var walk func(n *jnode)
	walk = func(n *jnode) {
		if n.kind != 's' {
			open[n.path] = on
		}
		for _, k := range n.children {
			walk(k)
		}
	}
	for _, k := range root.children {
		walk(k)
	}
}
