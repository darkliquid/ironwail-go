package renderer

import (
	"fmt"
	"image"
	"image/draw"
)

// TextureAtlasNode represents a node in the atlas tree
type TextureAtlasNode struct {
	rect  image.Rectangle
	child [2]*TextureAtlasNode
	image *image.RGBA // Non-nil if this node is filled
}

func newTextureAtlasNode(rect image.Rectangle) *TextureAtlasNode {
	return &TextureAtlasNode{rect: rect}
}

func (n *TextureAtlasNode) insert(img *image.RGBA) *TextureAtlasNode {
	if n.child[0] != nil || n.child[1] != nil {
		newNode := n.child[0].insert(img)
		if newNode != nil {
			return newNode
		}
		return n.child[1].insert(img)
	} else {
		if n.image != nil {
			return nil
		}
		imgBounds := img.Bounds()
		imgW, imgH := imgBounds.Dx(), imgBounds.Dy()
		nodeW, nodeH := n.rect.Dx(), n.rect.Dy()

		if imgW > nodeW || imgH > nodeH {
			return nil
		}
		if imgW == nodeW && imgH == nodeH {
			n.image = img
			return n
		}

		dw := nodeW - imgW
		dh := nodeH - imgH

		if dw > dh {
			n.child[0] = newTextureAtlasNode(image.Rect(n.rect.Min.X, n.rect.Min.Y, n.rect.Min.X+imgW, n.rect.Max.Y))
			n.child[1] = newTextureAtlasNode(image.Rect(n.rect.Min.X+imgW, n.rect.Min.Y, n.rect.Max.X, n.rect.Max.Y))
		} else {
			n.child[0] = newTextureAtlasNode(image.Rect(n.rect.Min.X, n.rect.Min.Y, n.rect.Max.X, n.rect.Min.Y+imgH))
			n.child[1] = newTextureAtlasNode(image.Rect(n.rect.Min.X, n.rect.Min.Y+imgH, n.rect.Max.X, n.rect.Max.Y))
		}
		return n.child[0].insert(img)
	}
}

// AtlasLayer represents a single layer in the texture array.
type AtlasLayer struct {
	root   *TextureAtlasNode
	width  int
	height int
	flat   *image.RGBA // lazily allocated by Flatten or used for direct draws
}

// WorldTextureAtlas manages an array of atlas layers
type WorldTextureAtlas struct {
	layers    []*AtlasLayer
	maxWidth  int
	maxHeight int
}

func NewWorldTextureAtlas(maxWidth, maxHeight int) *WorldTextureAtlas {
	return &WorldTextureAtlas{
		layers:    []*AtlasLayer{{root: newTextureAtlasNode(image.Rect(0, 0, maxWidth, maxHeight)), width: maxWidth, height: maxHeight}},
		maxWidth:  maxWidth,
		maxHeight: maxHeight,
	}
}

// AtlasInsertion records where a texture was placed in the atlas.
type AtlasInsertion struct {
	Layer int
	Rect  image.Rectangle
}

// InsertWithRect inserts an image and returns the insertion info including
// the pixel-space rectangle where it was placed.
func (a *WorldTextureAtlas) InsertWithRect(img *image.RGBA) (AtlasInsertion, float32, float32, float32, float32, error) {
	imgBounds := img.Bounds()
	imgW, imgH := imgBounds.Dx(), imgBounds.Dy()
	if imgW > a.maxWidth || imgH > a.maxHeight {
		return AtlasInsertion{}, 0, 0, 0, 0, fmt.Errorf("image too large for atlas")
	}

	for i, layer := range a.layers {
		node := layer.root.insert(img)
		if node != nil {
			u := float32(node.rect.Min.X) / float32(a.maxWidth)
			v := float32(node.rect.Min.Y) / float32(a.maxHeight)
			w := float32(imgW) / float32(a.maxWidth)
			h := float32(imgH) / float32(a.maxHeight)
			return AtlasInsertion{Layer: i, Rect: node.rect}, u, v, w, h, nil
		}
	}

	// No space found, create new layer
	newLayer := &AtlasLayer{
		root:   newTextureAtlasNode(image.Rect(0, 0, a.maxWidth, a.maxHeight)),
		width:  a.maxWidth,
		height: a.maxHeight,
	}
	a.layers = append(a.layers, newLayer)
	node := newLayer.root.insert(img)
	if node != nil {
		u := float32(node.rect.Min.X) / float32(a.maxWidth)
		v := float32(node.rect.Min.Y) / float32(a.maxHeight)
		w := float32(imgW) / float32(a.maxWidth)
		h := float32(imgH) / float32(a.maxHeight)
		return AtlasInsertion{Layer: len(a.layers) - 1, Rect: node.rect}, u, v, w, h, nil
	}

	return AtlasInsertion{}, 0, 0, 0, 0, fmt.Errorf("failed to insert image into new atlas layer")
}

// DrawAt draws an image at a specific layer and rect position previously
// returned by InsertWithRect. This lets companion atlases (e.g. fullbright)
// place data at the same position as their diffuse counterpart. The layer
// must already exist (call EnsureLayer first if needed).
func (a *WorldTextureAtlas) DrawAt(img *image.RGBA, ins AtlasInsertion) {
	if ins.Layer < 0 || ins.Layer >= len(a.layers) {
		return
	}
	layer := a.layers[ins.Layer]
	if layer.flat == nil {
		layer.flat = image.NewRGBA(image.Rect(0, 0, layer.width, layer.height))
	}
	draw.Draw(layer.flat, ins.Rect, img, img.Bounds().Min, draw.Src)
}

// EnsureLayerCount ensures the atlas has at least n layers, creating
// empty ones as needed. Used to keep companion atlases in sync.
func (a *WorldTextureAtlas) EnsureLayerCount(n int) {
	for len(a.layers) < n {
		a.layers = append(a.layers, &AtlasLayer{
			root:   newTextureAtlasNode(image.Rect(0, 0, a.maxWidth, a.maxHeight)),
			width:  a.maxWidth,
			height: a.maxHeight,
		})
	}
}

// Flatten generates a list of RGBA images for all layers
func (a *WorldTextureAtlas) Flatten() []*image.RGBA {
	results := make([]*image.RGBA, len(a.layers))
	for i, layer := range a.layers {
		// If a flat image was allocated (e.g. by DrawAt for companion data),
		// composite the tree-inserted textures onto it, otherwise start fresh.
		img := layer.flat
		if img == nil {
			img = image.NewRGBA(image.Rect(0, 0, layer.width, layer.height))
		}
		flattenNode(layer.root, img)
		results[i] = img
	}
	return results
}

func flattenNode(node *TextureAtlasNode, dst *image.RGBA) {
	if node == nil {
		return
	}
	if node.image != nil {
		draw.Draw(dst, node.rect, node.image, node.image.Bounds().Min, draw.Src)
	}
	if node.child[0] != nil {
		flattenNode(node.child[0], dst)
	}
	if node.child[1] != nil {
		flattenNode(node.child[1], dst)
	}
}
