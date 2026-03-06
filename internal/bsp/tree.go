package bsp

import (
	"encoding/binary"
	"fmt"
	"io"
)

type TreeFace struct {
	PlaneNum  int32
	Side      int32
	FirstEdge int32
	NumEdges  int32
	Texinfo   int32
	Styles    [MaxLightmaps]uint8
	LightOfs  int32
}

type TreeEdge struct {
	V [2]uint32
}

type TreeChild struct {
	IsLeaf bool
	Index  int
}

type TreeNode struct {
	PlaneNum  int32
	BoundsMin [3]float32
	BoundsMax [3]float32
	Children  [2]TreeChild
	FirstFace uint32
	NumFaces  uint32
	Parent    int
}

type TreeLeaf struct {
	Contents         int32
	VisOfs           int32
	BoundsMin        [3]float32
	BoundsMax        [3]float32
	FirstMarkSurface uint32
	NumMarkSurfaces  uint32
	AmbientLevel     [NumAmbients]uint8
	Parent           int
}

type Tree struct {
	Header  DHeader
	Version int32

	Entities    []byte
	Visibility  []byte
	TextureData []byte
	Lighting    []byte

	Planes    []DPlane
	Vertexes  []DVertex
	Texinfo   []Texinfo
	Edges     []TreeEdge
	Surfedges []int32
	Faces     []TreeFace

	MarkSurfaces []int
	Leafs        []TreeLeaf
	Nodes        []TreeNode
	Models       []DModel

	NumTextures int32
}

const (
	dPlaneSize  = 20
	dVertexSize = 12
	dsEdgeSize  = 4
	dlEdgeSize  = 8
	dsFaceSize  = 20
	dlFaceSize  = 28
	dsLeafSize  = 28
	dl1LeafSize = 32
	dl2LeafSize = 44
	dsNodeSize  = 24
	dl1NodeSize = 28
	dl2NodeSize = 44
	dModelSize  = 64
	int32Size   = 4
	uint16Size  = 2
	uint32Size  = 4
)

func LoadTree(r io.ReadSeeker) (*Tree, error) {
	reader := NewReader(r)

	header, err := reader.ReadHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read BSP header: %w", err)
	}
	if !IsValidVersion(header.Version) {
		return nil, fmt.Errorf("unsupported BSP version: %d", header.Version)
	}

	t := &Tree{Header: *header, Version: header.Version}

	if t.Entities, err = reader.ReadLump(&header.Lumps[LumpEntities]); err != nil {
		return nil, fmt.Errorf("load entities: %w", err)
	}
	if t.Visibility, err = reader.ReadLump(&header.Lumps[LumpVisibility]); err != nil {
		return nil, fmt.Errorf("load visibility: %w", err)
	}
	if err := t.loadTextures(reader); err != nil {
		return nil, err
	}
	if t.Lighting, err = reader.ReadLump(&header.Lumps[LumpLighting]); err != nil {
		return nil, fmt.Errorf("load lighting: %w", err)
	}

	if err := t.loadPlanes(reader); err != nil {
		return nil, err
	}
	if err := t.loadVertexes(reader); err != nil {
		return nil, err
	}
	if err := t.loadTexinfo(reader); err != nil {
		return nil, err
	}
	if err := t.loadEdges(reader); err != nil {
		return nil, err
	}
	if err := t.loadSurfedges(reader); err != nil {
		return nil, err
	}
	if err := t.loadFaces(reader); err != nil {
		return nil, err
	}
	if err := t.loadMarkSurfaces(reader); err != nil {
		return nil, err
	}
	if err := t.loadLeafs(reader); err != nil {
		return nil, err
	}
	if err := t.loadNodes(reader); err != nil {
		return nil, err
	}
	if err := t.loadModels(reader); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Tree) loadPlanes(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpPlanes])
	if err != nil {
		return fmt.Errorf("load planes lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if len(data)%dPlaneSize != 0 {
		return fmt.Errorf("load planes: funny lump size %d", len(data))
	}

	t.Planes = make([]DPlane, len(data)/dPlaneSize)
	for i := range t.Planes {
		o := i * dPlaneSize
		t.Planes[i] = DPlane{
			Normal: [3]float32{
				Float32frombits(binary.LittleEndian.Uint32(data[o:])),
				Float32frombits(binary.LittleEndian.Uint32(data[o+4:])),
				Float32frombits(binary.LittleEndian.Uint32(data[o+8:])),
			},
			Dist: Float32frombits(binary.LittleEndian.Uint32(data[o+12:])),
			Type: int32(binary.LittleEndian.Uint32(data[o+16:])),
		}
	}
	return nil
}

func (t *Tree) loadTextures(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpTextures])
	if err != nil {
		return fmt.Errorf("load textures lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if len(data) >= 4 {
		t.NumTextures = int32(binary.LittleEndian.Uint32(data[0:4]))
	}
	t.TextureData = data
	return nil
}

func (t *Tree) loadVertexes(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpVertexes])
	if err != nil {
		return fmt.Errorf("load vertexes lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if len(data)%dVertexSize != 0 {
		return fmt.Errorf("load vertexes: funny lump size %d", len(data))
	}

	t.Vertexes = make([]DVertex, len(data)/dVertexSize)
	for i := range t.Vertexes {
		o := i * dVertexSize
		t.Vertexes[i] = DVertex{
			Point: [3]float32{
				Float32frombits(binary.LittleEndian.Uint32(data[o:])),
				Float32frombits(binary.LittleEndian.Uint32(data[o+4:])),
				Float32frombits(binary.LittleEndian.Uint32(data[o+8:])),
			},
		}
	}
	return nil
}

func (t *Tree) loadTexinfo(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpTexinfo])
	if err != nil {
		return fmt.Errorf("load texinfo lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if len(data)%40 != 0 {
		return fmt.Errorf("load texinfo: funny lump size %d", len(data))
	}

	t.Texinfo = make([]Texinfo, len(data)/40)
	for i := range t.Texinfo {
		o := i * 40
		var ti Texinfo
		for j := 0; j < 2; j++ {
			for k := 0; k < 4; k++ {
				ti.Vecs[j][k] = Float32frombits(binary.LittleEndian.Uint32(data[o+j*16+k*4:]))
			}
		}
		ti.Miptex = int32(binary.LittleEndian.Uint32(data[o+32:]))
		ti.Flags = int32(binary.LittleEndian.Uint32(data[o+36:]))
		t.Texinfo[i] = ti
	}
	return nil
}

func (t *Tree) loadEdges(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpEdges])
	if err != nil {
		return fmt.Errorf("load edges lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	if IsBSP2(t.Version) {
		if len(data)%dlEdgeSize != 0 {
			return fmt.Errorf("load edges: funny lump size %d", len(data))
		}
		t.Edges = make([]TreeEdge, len(data)/dlEdgeSize)
		for i := range t.Edges {
			o := i * dlEdgeSize
			t.Edges[i] = TreeEdge{
				V: [2]uint32{
					binary.LittleEndian.Uint32(data[o:]),
					binary.LittleEndian.Uint32(data[o+4:]),
				},
			}
		}
		return nil
	}

	if len(data)%dsEdgeSize != 0 {
		return fmt.Errorf("load edges: funny lump size %d", len(data))
	}
	t.Edges = make([]TreeEdge, len(data)/dsEdgeSize)
	for i := range t.Edges {
		o := i * dsEdgeSize
		t.Edges[i] = TreeEdge{
			V: [2]uint32{
				uint32(binary.LittleEndian.Uint16(data[o:])),
				uint32(binary.LittleEndian.Uint16(data[o+2:])),
			},
		}
	}
	return nil
}

func (t *Tree) loadSurfedges(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpSurfedges])
	if err != nil {
		return fmt.Errorf("load surfedges lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if len(data)%int32Size != 0 {
		return fmt.Errorf("load surfedges: funny lump size %d", len(data))
	}

	t.Surfedges = make([]int32, len(data)/int32Size)
	for i := range t.Surfedges {
		t.Surfedges[i] = int32(binary.LittleEndian.Uint32(data[i*int32Size:]))
	}
	return nil
}

func (t *Tree) loadFaces(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpFaces])
	if err != nil {
		return fmt.Errorf("load faces lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	if IsBSP2(t.Version) {
		if len(data)%dlFaceSize != 0 {
			return fmt.Errorf("load faces: funny lump size %d", len(data))
		}
		t.Faces = make([]TreeFace, len(data)/dlFaceSize)
		for i := range t.Faces {
			o := i * dlFaceSize
			t.Faces[i] = TreeFace{
				PlaneNum:  int32(binary.LittleEndian.Uint32(data[o:])),
				Side:      int32(binary.LittleEndian.Uint32(data[o+4:])),
				FirstEdge: int32(binary.LittleEndian.Uint32(data[o+8:])),
				NumEdges:  int32(binary.LittleEndian.Uint32(data[o+12:])),
				Texinfo:   int32(binary.LittleEndian.Uint32(data[o+16:])),
				LightOfs:  int32(binary.LittleEndian.Uint32(data[o+24:])),
			}
			copy(t.Faces[i].Styles[:], data[o+20:o+24])
		}
		return nil
	}

	if len(data)%dsFaceSize != 0 {
		return fmt.Errorf("load faces: funny lump size %d", len(data))
	}
	t.Faces = make([]TreeFace, len(data)/dsFaceSize)
	for i := range t.Faces {
		o := i * dsFaceSize
		t.Faces[i] = TreeFace{
			PlaneNum:  int32(binary.LittleEndian.Uint16(data[o:])),
			Side:      int32(binary.LittleEndian.Uint16(data[o+2:])),
			FirstEdge: int32(binary.LittleEndian.Uint32(data[o+4:])),
			NumEdges:  int32(binary.LittleEndian.Uint16(data[o+8:])),
			Texinfo:   int32(binary.LittleEndian.Uint16(data[o+10:])),
			LightOfs:  int32(binary.LittleEndian.Uint32(data[o+16:])),
		}
		copy(t.Faces[i].Styles[:], data[o+12:o+16])
	}
	return nil
}

func (t *Tree) loadMarkSurfaces(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpMarksurfaces])
	if err != nil {
		return fmt.Errorf("load marksurfaces lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	if IsBSP2(t.Version) {
		if len(data)%uint32Size != 0 {
			return fmt.Errorf("load marksurfaces: funny lump size %d", len(data))
		}
		t.MarkSurfaces = make([]int, len(data)/uint32Size)
		for i := range t.MarkSurfaces {
			j := int(binary.LittleEndian.Uint32(data[i*uint32Size:]))
			if j < 0 || j >= len(t.Faces) {
				return fmt.Errorf("load marksurfaces: bad surface number %d", j)
			}
			t.MarkSurfaces[i] = j
		}
		return nil
	}

	if len(data)%uint16Size != 0 {
		return fmt.Errorf("load marksurfaces: funny lump size %d", len(data))
	}
	t.MarkSurfaces = make([]int, len(data)/uint16Size)
	for i := range t.MarkSurfaces {
		j := int(binary.LittleEndian.Uint16(data[i*uint16Size:]))
		if j < 0 || j >= len(t.Faces) {
			return fmt.Errorf("load marksurfaces: bad surface number %d", j)
		}
		t.MarkSurfaces[i] = j
	}
	return nil
}

func (t *Tree) loadLeafs(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpLeafs])
	if err != nil {
		return fmt.Errorf("load leafs lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	if IsBSP2(t.Version) {
		if t.Version == BSP2Version_BSP2 {
			if len(data)%dl2LeafSize != 0 {
				return fmt.Errorf("load leafs: funny lump size %d", len(data))
			}
			t.Leafs = make([]TreeLeaf, len(data)/dl2LeafSize)
			for i := range t.Leafs {
				o := i * dl2LeafSize
				leaf := &t.Leafs[i]
				leaf.Contents = int32(binary.LittleEndian.Uint32(data[o:]))
				leaf.VisOfs = int32(binary.LittleEndian.Uint32(data[o+4:]))
				leaf.BoundsMin = [3]float32{
					Float32frombits(binary.LittleEndian.Uint32(data[o+8:])),
					Float32frombits(binary.LittleEndian.Uint32(data[o+12:])),
					Float32frombits(binary.LittleEndian.Uint32(data[o+16:])),
				}
				leaf.BoundsMax = [3]float32{
					Float32frombits(binary.LittleEndian.Uint32(data[o+20:])),
					Float32frombits(binary.LittleEndian.Uint32(data[o+24:])),
					Float32frombits(binary.LittleEndian.Uint32(data[o+28:])),
				}
				leaf.FirstMarkSurface = binary.LittleEndian.Uint32(data[o+32:])
				leaf.NumMarkSurfaces = binary.LittleEndian.Uint32(data[o+36:])
				copy(leaf.AmbientLevel[:], data[o+40:o+44])
				leaf.Parent = -1
				if err := t.validateLeafMarkSurfaceRange(i, leaf.FirstMarkSurface, leaf.NumMarkSurfaces); err != nil {
					return err
				}
			}
			return nil
		}

		if len(data)%dl1LeafSize != 0 {
			return fmt.Errorf("load leafs: funny lump size %d", len(data))
		}
		t.Leafs = make([]TreeLeaf, len(data)/dl1LeafSize)
		for i := range t.Leafs {
			o := i * dl1LeafSize
			leaf := &t.Leafs[i]
			leaf.Contents = int32(binary.LittleEndian.Uint32(data[o:]))
			leaf.VisOfs = int32(binary.LittleEndian.Uint32(data[o+4:]))
			leaf.BoundsMin = [3]float32{
				float32(int16(binary.LittleEndian.Uint16(data[o+8:]))),
				float32(int16(binary.LittleEndian.Uint16(data[o+10:]))),
				float32(int16(binary.LittleEndian.Uint16(data[o+12:]))),
			}
			leaf.BoundsMax = [3]float32{
				float32(int16(binary.LittleEndian.Uint16(data[o+14:]))),
				float32(int16(binary.LittleEndian.Uint16(data[o+16:]))),
				float32(int16(binary.LittleEndian.Uint16(data[o+18:]))),
			}
			leaf.FirstMarkSurface = binary.LittleEndian.Uint32(data[o+20:])
			leaf.NumMarkSurfaces = binary.LittleEndian.Uint32(data[o+24:])
			copy(leaf.AmbientLevel[:], data[o+28:o+32])
			leaf.Parent = -1
			if err := t.validateLeafMarkSurfaceRange(i, leaf.FirstMarkSurface, leaf.NumMarkSurfaces); err != nil {
				return err
			}
		}
		return nil
	}

	if len(data)%dsLeafSize != 0 {
		return fmt.Errorf("load leafs: funny lump size %d", len(data))
	}
	t.Leafs = make([]TreeLeaf, len(data)/dsLeafSize)
	for i := range t.Leafs {
		o := i * dsLeafSize
		leaf := &t.Leafs[i]
		leaf.Contents = int32(binary.LittleEndian.Uint32(data[o:]))
		leaf.VisOfs = int32(binary.LittleEndian.Uint32(data[o+4:]))
		leaf.BoundsMin = [3]float32{
			float32(int16(binary.LittleEndian.Uint16(data[o+8:]))),
			float32(int16(binary.LittleEndian.Uint16(data[o+10:]))),
			float32(int16(binary.LittleEndian.Uint16(data[o+12:]))),
		}
		leaf.BoundsMax = [3]float32{
			float32(int16(binary.LittleEndian.Uint16(data[o+14:]))),
			float32(int16(binary.LittleEndian.Uint16(data[o+16:]))),
			float32(int16(binary.LittleEndian.Uint16(data[o+18:]))),
		}
		leaf.FirstMarkSurface = uint32(binary.LittleEndian.Uint16(data[o+20:]))
		leaf.NumMarkSurfaces = uint32(binary.LittleEndian.Uint16(data[o+22:]))
		copy(leaf.AmbientLevel[:], data[o+24:o+28])
		leaf.Parent = -1
		if err := t.validateLeafMarkSurfaceRange(i, leaf.FirstMarkSurface, leaf.NumMarkSurfaces); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tree) validateLeafMarkSurfaceRange(leafIndex int, first, count uint32) error {
	if count == 0 {
		return nil
	}
	if int(first) >= len(t.MarkSurfaces) {
		return fmt.Errorf("load leafs: leaf %d marksurface start out of bounds", leafIndex)
	}
	end := uint64(first) + uint64(count)
	if end > uint64(len(t.MarkSurfaces)) {
		return fmt.Errorf("load leafs: leaf %d marksurface range out of bounds", leafIndex)
	}
	return nil
}

func (t *Tree) loadNodes(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpNodes])
	if err != nil {
		return fmt.Errorf("load nodes lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	if IsBSP2(t.Version) {
		if t.Version == BSP2Version_BSP2 {
			if len(data)%dl2NodeSize != 0 {
				return fmt.Errorf("load nodes: funny lump size %d", len(data))
			}
			t.Nodes = make([]TreeNode, len(data)/dl2NodeSize)
			for i := range t.Nodes {
				o := i * dl2NodeSize
				n := &t.Nodes[i]
				n.PlaneNum = int32(binary.LittleEndian.Uint32(data[o:]))
				n.BoundsMin = [3]float32{
					Float32frombits(binary.LittleEndian.Uint32(data[o+12:])),
					Float32frombits(binary.LittleEndian.Uint32(data[o+16:])),
					Float32frombits(binary.LittleEndian.Uint32(data[o+20:])),
				}
				n.BoundsMax = [3]float32{
					Float32frombits(binary.LittleEndian.Uint32(data[o+24:])),
					Float32frombits(binary.LittleEndian.Uint32(data[o+28:])),
					Float32frombits(binary.LittleEndian.Uint32(data[o+32:])),
				}
				n.FirstFace = binary.LittleEndian.Uint32(data[o+36:])
				n.NumFaces = binary.LittleEndian.Uint32(data[o+40:])
				n.Parent = -1

				if err := t.validateNode(i, n); err != nil {
					return err
				}

				for j := 0; j < 2; j++ {
					child := int32(binary.LittleEndian.Uint32(data[o+4+j*4:]))
					ref, err := t.resolveBSP2NodeChild(len(t.Nodes), child)
					if err != nil {
						return fmt.Errorf("load nodes: node %d child %d: %w", i, j, err)
					}
					n.Children[j] = ref
				}
			}
			t.setParents()
			return nil
		}

		if len(data)%dl1NodeSize != 0 {
			return fmt.Errorf("load nodes: funny lump size %d", len(data))
		}
		t.Nodes = make([]TreeNode, len(data)/dl1NodeSize)
		for i := range t.Nodes {
			o := i * dl1NodeSize
			n := &t.Nodes[i]
			n.PlaneNum = int32(binary.LittleEndian.Uint32(data[o:]))
			n.BoundsMin = [3]float32{
				float32(int16(binary.LittleEndian.Uint16(data[o+12:]))),
				float32(int16(binary.LittleEndian.Uint16(data[o+14:]))),
				float32(int16(binary.LittleEndian.Uint16(data[o+16:]))),
			}
			n.BoundsMax = [3]float32{
				float32(int16(binary.LittleEndian.Uint16(data[o+18:]))),
				float32(int16(binary.LittleEndian.Uint16(data[o+20:]))),
				float32(int16(binary.LittleEndian.Uint16(data[o+22:]))),
			}
			n.FirstFace = binary.LittleEndian.Uint32(data[o+20:])
			n.NumFaces = binary.LittleEndian.Uint32(data[o+24:])
			n.Parent = -1

			if err := t.validateNode(i, n); err != nil {
				return err
			}

			for j := 0; j < 2; j++ {
				child := int32(binary.LittleEndian.Uint32(data[o+4+j*4:]))
				ref, err := t.resolveBSP2NodeChild(len(t.Nodes), child)
				if err != nil {
					return fmt.Errorf("load nodes: node %d child %d: %w", i, j, err)
				}
				n.Children[j] = ref
			}
		}
		t.setParents()
		return nil
	}

	if len(data)%dsNodeSize != 0 {
		return fmt.Errorf("load nodes: funny lump size %d", len(data))
	}
	t.Nodes = make([]TreeNode, len(data)/dsNodeSize)
	for i := range t.Nodes {
		o := i * dsNodeSize
		n := &t.Nodes[i]
		n.PlaneNum = int32(binary.LittleEndian.Uint32(data[o:]))
		n.BoundsMin = [3]float32{
			float32(int16(binary.LittleEndian.Uint16(data[o+8:]))),
			float32(int16(binary.LittleEndian.Uint16(data[o+10:]))),
			float32(int16(binary.LittleEndian.Uint16(data[o+12:]))),
		}
		n.BoundsMax = [3]float32{
			float32(int16(binary.LittleEndian.Uint16(data[o+14:]))),
			float32(int16(binary.LittleEndian.Uint16(data[o+16:]))),
			float32(int16(binary.LittleEndian.Uint16(data[o+18:]))),
		}
		n.FirstFace = uint32(binary.LittleEndian.Uint16(data[o+20:]))
		n.NumFaces = uint32(binary.LittleEndian.Uint16(data[o+22:]))
		n.Parent = -1

		if err := t.validateNode(i, n); err != nil {
			return err
		}

		for j := 0; j < 2; j++ {
			raw := binary.LittleEndian.Uint16(data[o+4+j*2:])
			ref, err := t.resolveStandardNodeChild(len(t.Nodes), raw)
			if err != nil {
				return fmt.Errorf("load nodes: node %d child %d: %w", i, j, err)
			}
			n.Children[j] = ref
		}
	}
	t.setParents()
	return nil
}

func (t *Tree) validateNode(nodeIndex int, n *TreeNode) error {
	if n.PlaneNum < 0 || int(n.PlaneNum) >= len(t.Planes) {
		return fmt.Errorf("node %d has invalid plane index %d", nodeIndex, n.PlaneNum)
	}
	if n.NumFaces > 0 {
		if int(n.FirstFace) >= len(t.Faces) {
			return fmt.Errorf("node %d has first face out of bounds", nodeIndex)
		}
		if uint64(n.FirstFace)+uint64(n.NumFaces) > uint64(len(t.Faces)) {
			return fmt.Errorf("node %d has face range out of bounds", nodeIndex)
		}
	}
	return nil
}

func (t *Tree) resolveStandardNodeChild(nodeCount int, raw uint16) (TreeChild, error) {
	p := int(raw)
	if p < nodeCount {
		return TreeChild{Index: p}, nil
	}
	leaf := 65535 - p
	if leaf < 0 || leaf >= len(t.Leafs) {
		return TreeChild{}, fmt.Errorf("invalid leaf index %d (file has only %d leafs)", leaf, len(t.Leafs))
	}
	return TreeChild{IsLeaf: true, Index: leaf}, nil
}

func (t *Tree) resolveBSP2NodeChild(nodeCount int, raw int32) (TreeChild, error) {
	if raw >= 0 {
		p := int(raw)
		if p < nodeCount {
			return TreeChild{Index: p}, nil
		}
		return TreeChild{}, fmt.Errorf("invalid node index %d (file has only %d nodes)", p, nodeCount)
	}
	leaf := int(^raw)
	if leaf < 0 || leaf >= len(t.Leafs) {
		return TreeChild{}, fmt.Errorf("invalid leaf index %d (file has only %d leafs)", leaf, len(t.Leafs))
	}
	return TreeChild{IsLeaf: true, Index: leaf}, nil
}

func (t *Tree) setParents() {
	if len(t.Nodes) == 0 {
		return
	}
	t.Nodes[0].Parent = -1
	for i := range t.Nodes {
		for _, child := range t.Nodes[i].Children {
			if child.IsLeaf {
				if child.Index >= 0 && child.Index < len(t.Leafs) {
					t.Leafs[child.Index].Parent = i
				}
				continue
			}
			if child.Index >= 0 && child.Index < len(t.Nodes) {
				t.Nodes[child.Index].Parent = i
			}
		}
	}
}

func (t *Tree) loadModels(r *Reader) error {
	data, err := r.ReadLump(&t.Header.Lumps[LumpModels])
	if err != nil {
		return fmt.Errorf("load models lump: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if len(data)%dModelSize != 0 {
		return fmt.Errorf("load models: funny lump size %d", len(data))
	}

	t.Models = make([]DModel, len(data)/dModelSize)
	for i := range t.Models {
		o := i * dModelSize
		m := &t.Models[i]
		m.BoundsMin = [3]float32{
			Float32frombits(binary.LittleEndian.Uint32(data[o:])),
			Float32frombits(binary.LittleEndian.Uint32(data[o+4:])),
			Float32frombits(binary.LittleEndian.Uint32(data[o+8:])),
		}
		m.BoundsMax = [3]float32{
			Float32frombits(binary.LittleEndian.Uint32(data[o+12:])),
			Float32frombits(binary.LittleEndian.Uint32(data[o+16:])),
			Float32frombits(binary.LittleEndian.Uint32(data[o+20:])),
		}
		m.Origin = [3]float32{
			Float32frombits(binary.LittleEndian.Uint32(data[o+24:])),
			Float32frombits(binary.LittleEndian.Uint32(data[o+28:])),
			Float32frombits(binary.LittleEndian.Uint32(data[o+32:])),
		}
		for j := 0; j < MaxMapHulls; j++ {
			m.HeadNode[j] = int32(binary.LittleEndian.Uint32(data[o+36+j*4:]))
		}
		m.VisLeafs = int32(binary.LittleEndian.Uint32(data[o+52:]))
		m.FirstFace = int32(binary.LittleEndian.Uint32(data[o+56:]))
		m.NumFaces = int32(binary.LittleEndian.Uint32(data[o+60:]))

		if m.FirstFace < 0 || m.NumFaces < 0 {
			return fmt.Errorf("load models: model %d has negative face span", i)
		}
		if m.NumFaces > 0 {
			if int(m.FirstFace) >= len(t.Faces) {
				return fmt.Errorf("load models: model %d first face out of bounds", i)
			}
			if uint64(m.FirstFace)+uint64(m.NumFaces) > uint64(len(t.Faces)) {
				return fmt.Errorf("load models: model %d face range out of bounds", i)
			}
		}
	}

	return nil
}
