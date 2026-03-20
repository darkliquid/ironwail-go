package bsp

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Load reads and parses a complete BSP file from the reader.
// It returns a File struct containing all the parsed data.
func Load(r io.ReadSeeker) (*File, error) {
	reader := NewReader(r)

	header, err := reader.ReadHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read BSP header: %w", err)
	}

	if !IsValidVersion(header.Version) {
		return nil, fmt.Errorf("unsupported BSP version: %d", header.Version)
	}

	file := &File{
		Header:    *header,
		Version:   header.Version,
		IsBSP2:    IsBSP2(header.Version),
		IsQuake64: IsQuake64(header.Version),
	}

	if err := file.loadEntities(reader); err != nil {
		return nil, fmt.Errorf("failed to load entities: %w", err)
	}
	if err := file.loadPlanes(reader); err != nil {
		return nil, fmt.Errorf("failed to load planes: %w", err)
	}
	if err := file.loadVertexes(reader); err != nil {
		return nil, fmt.Errorf("failed to load vertexes: %w", err)
	}
	if err := file.loadVisibility(reader); err != nil {
		return nil, fmt.Errorf("failed to load visibility: %w", err)
	}
	if err := file.loadNodes(reader); err != nil {
		return nil, fmt.Errorf("failed to load nodes: %w", err)
	}
	if err := file.loadTexinfo(reader); err != nil {
		return nil, fmt.Errorf("failed to load texinfo: %w", err)
	}
	if err := file.loadFaces(reader); err != nil {
		return nil, fmt.Errorf("failed to load faces: %w", err)
	}
	if err := file.loadLighting(reader); err != nil {
		return nil, fmt.Errorf("failed to load lighting: %w", err)
	}
	if err := file.loadClipnodes(reader); err != nil {
		return nil, fmt.Errorf("failed to load clipnodes: %w", err)
	}
	if err := file.loadLeafs(reader); err != nil {
		return nil, fmt.Errorf("failed to load leafs: %w", err)
	}
	if err := file.loadMarkSurfaces(reader); err != nil {
		return nil, fmt.Errorf("failed to load marksurfaces: %w", err)
	}
	if err := file.loadEdges(reader); err != nil {
		return nil, fmt.Errorf("failed to load edges: %w", err)
	}
	if err := file.loadSurfedges(reader); err != nil {
		return nil, fmt.Errorf("failed to load surfedges: %w", err)
	}
	if err := file.loadModels(reader); err != nil {
		return nil, fmt.Errorf("failed to load models: %w", err)
	}
	if err := file.loadTextures(reader); err != nil {
		return nil, fmt.Errorf("failed to load textures: %w", err)
	}

	return file, nil
}

func (f *File) loadEntities(r *Reader) error {
	data, err := r.ReadLump(&f.Header.Lumps[LumpEntities])
	if err != nil {
		return err
	}
	f.Entities = data
	return nil
}

func (f *File) loadPlanes(r *Reader) error {
	lump := &f.Header.Lumps[LumpPlanes]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	count := int(lump.FileLength) / 20 // DPlane is 20 bytes
	f.Planes = make([]DPlane, count)

	for i := 0; i < count; i++ {
		offset := i * 20
		f.Planes[i] = DPlane{
			Normal: [3]float32{
				Float32frombits(binary.LittleEndian.Uint32(data[offset:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+4:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+8:])),
			},
			Dist: Float32frombits(binary.LittleEndian.Uint32(data[offset+12:])),
			Type: int32(binary.LittleEndian.Uint32(data[offset+16:])),
		}
	}
	return nil
}

func (f *File) loadVertexes(r *Reader) error {
	lump := &f.Header.Lumps[LumpVertexes]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	count := int(lump.FileLength) / 12 // DVertex is 12 bytes
	f.Vertexes = make([]DVertex, count)

	for i := 0; i < count; i++ {
		offset := i * 12
		f.Vertexes[i] = DVertex{
			Point: [3]float32{
				Float32frombits(binary.LittleEndian.Uint32(data[offset:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+4:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+8:])),
			},
		}
	}
	return nil
}

func (f *File) loadVisibility(r *Reader) error {
	data, err := r.ReadLump(&f.Header.Lumps[LumpVisibility])
	if err != nil {
		return err
	}
	f.Visibility = data
	return nil
}

func (f *File) loadNodes(r *Reader) error {
	lump := &f.Header.Lumps[LumpNodes]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		if f.Version == BSP2Version_BSP2 {
			count := int(lump.FileLength) / 36 // DL2Node is 36 bytes
			nodes := make([]DL2Node, count)
			for i := 0; i < count; i++ {
				offset := i * 36
				nodes[i] = DL2Node{
					PlaneNum: int32(binary.LittleEndian.Uint32(data[offset:])),
					Children: [2]int32{
						int32(binary.LittleEndian.Uint32(data[offset+4:])),
						int32(binary.LittleEndian.Uint32(data[offset+8:])),
					},
					BoundsMin: [3]float32{
						Float32frombits(binary.LittleEndian.Uint32(data[offset+12:])),
						Float32frombits(binary.LittleEndian.Uint32(data[offset+16:])),
						Float32frombits(binary.LittleEndian.Uint32(data[offset+20:])),
					},
					BoundsMax: [3]float32{
						Float32frombits(binary.LittleEndian.Uint32(data[offset+24:])),
						Float32frombits(binary.LittleEndian.Uint32(data[offset+28:])),
						Float32frombits(binary.LittleEndian.Uint32(data[offset+32:])),
					},
				}
				// FirstFace and NumFaces would be at offset+36 and offset+40, but lump size check needed
			}
			f.Nodes = nodes
		} else {
			count := int(lump.FileLength) / 28 // DL1Node is 28 bytes
			nodes := make([]DL1Node, count)
			for i := 0; i < count; i++ {
				offset := i * 28
				nodes[i] = DL1Node{
					PlaneNum: int32(binary.LittleEndian.Uint32(data[offset:])),
					Children: [2]int32{
						int32(binary.LittleEndian.Uint32(data[offset+4:])),
						int32(binary.LittleEndian.Uint32(data[offset+8:])),
					},
					BoundsMin: [3]int16{
						int16(binary.LittleEndian.Uint16(data[offset+12:])),
						int16(binary.LittleEndian.Uint16(data[offset+14:])),
						int16(binary.LittleEndian.Uint16(data[offset+16:])),
					},
					BoundsMax: [3]int16{
						int16(binary.LittleEndian.Uint16(data[offset+18:])),
						int16(binary.LittleEndian.Uint16(data[offset+20:])),
						int16(binary.LittleEndian.Uint16(data[offset+22:])),
					},
				}
			}
			f.Nodes = nodes
		}
	} else {
		if len(data)%24 != 0 {
			return fmt.Errorf("load nodes: funny lump size %d", len(data))
		}
		count := int(lump.FileLength) / 24 // DSNode is 24 bytes
		nodes := make([]DSNode, count)
		for i := 0; i < count; i++ {
			offset := i * 24
			nodes[i] = DSNode{
				PlaneNum: int32(binary.LittleEndian.Uint32(data[offset:])),
				Children: [2]int16{
					int16(binary.LittleEndian.Uint16(data[offset+4:])),
					int16(binary.LittleEndian.Uint16(data[offset+6:])),
				},
				BoundsMin: [3]int16{
					int16(binary.LittleEndian.Uint16(data[offset+8:])),
					int16(binary.LittleEndian.Uint16(data[offset+10:])),
					int16(binary.LittleEndian.Uint16(data[offset+12:])),
				},
				BoundsMax: [3]int16{
					int16(binary.LittleEndian.Uint16(data[offset+14:])),
					int16(binary.LittleEndian.Uint16(data[offset+16:])),
					int16(binary.LittleEndian.Uint16(data[offset+18:])),
				},
				FirstFace: binary.LittleEndian.Uint16(data[offset+20:]),
				NumFaces:  binary.LittleEndian.Uint16(data[offset+22:]),
			}
		}
		f.Nodes = nodes
	}
	return nil
}

func (f *File) loadTexinfo(r *Reader) error {
	lump := &f.Header.Lumps[LumpTexinfo]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	count := int(lump.FileLength) / 40 // Texinfo is 40 bytes
	f.Texinfo = make([]Texinfo, count)

	for i := 0; i < count; i++ {
		offset := i * 40
		var ti Texinfo
		for j := 0; j < 2; j++ {
			for k := 0; k < 4; k++ {
				ti.Vecs[j][k] = Float32frombits(binary.LittleEndian.Uint32(data[offset+j*16+k*4:]))
			}
		}
		ti.Miptex = int32(binary.LittleEndian.Uint32(data[offset+32:]))
		ti.Flags = int32(binary.LittleEndian.Uint32(data[offset+36:]))
		f.Texinfo[i] = ti
	}
	return nil
}

func (f *File) loadFaces(r *Reader) error {
	lump := &f.Header.Lumps[LumpFaces]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		count := int(lump.FileLength) / 28 // DLFace is 28 bytes
		faces := make([]DLFace, count)
		for i := 0; i < count; i++ {
			offset := i * 28
			faces[i] = DLFace{
				PlaneNum:  int32(binary.LittleEndian.Uint32(data[offset:])),
				Side:      int32(binary.LittleEndian.Uint32(data[offset+4:])),
				FirstEdge: int32(binary.LittleEndian.Uint32(data[offset+8:])),
				NumEdges:  int32(binary.LittleEndian.Uint32(data[offset+12:])),
				Texinfo:   int32(binary.LittleEndian.Uint32(data[offset+16:])),
				LightOfs:  int32(binary.LittleEndian.Uint32(data[offset+24:])),
			}
			copy(faces[i].Styles[:], data[offset+20:offset+24])
		}
		f.Faces = faces
	} else {
		count := int(lump.FileLength) / 20 // DSFace is 20 bytes
		faces := make([]DSFace, count)
		for i := 0; i < count; i++ {
			offset := i * 20
			faces[i] = DSFace{
				PlaneNum:  int16(binary.LittleEndian.Uint16(data[offset:])),
				Side:      int16(binary.LittleEndian.Uint16(data[offset+2:])),
				FirstEdge: int32(binary.LittleEndian.Uint32(data[offset+4:])),
				NumEdges:  int16(binary.LittleEndian.Uint16(data[offset+8:])),
				Texinfo:   int16(binary.LittleEndian.Uint16(data[offset+10:])),
				LightOfs:  int32(binary.LittleEndian.Uint32(data[offset+16:])),
			}
			copy(faces[i].Styles[:], data[offset+12:offset+16])
		}
		f.Faces = faces
	}
	return nil
}

func (f *File) loadLighting(r *Reader) error {
	data, err := r.ReadLump(&f.Header.Lumps[LumpLighting])
	if err != nil {
		return err
	}
	f.Lighting = data
	return nil
}

func (f *File) loadClipnodes(r *Reader) error {
	lump := &f.Header.Lumps[LumpClipnodes]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		count := int(lump.FileLength) / 12 // DLClipNode is 12 bytes
		clipnodes := make([]DLClipNode, count)
		for i := 0; i < count; i++ {
			offset := i * 12
			clipnodes[i] = DLClipNode{
				PlaneNum: int32(binary.LittleEndian.Uint32(data[offset:])),
				Children: [2]int32{
					int32(binary.LittleEndian.Uint32(data[offset+4:])),
					int32(binary.LittleEndian.Uint32(data[offset+8:])),
				},
			}
		}
		f.Clipnodes = clipnodes
	} else {
		count := int(lump.FileLength) / 8 // DSClipNode is 8 bytes
		clipnodes := make([]DSClipNode, count)
		for i := 0; i < count; i++ {
			offset := i * 8
			clipnodes[i] = DSClipNode{
				PlaneNum: int32(binary.LittleEndian.Uint32(data[offset:])),
				Children: [2]int16{
					int16(binary.LittleEndian.Uint16(data[offset+4:])),
					int16(binary.LittleEndian.Uint16(data[offset+6:])),
				},
			}
		}
		f.Clipnodes = clipnodes
	}
	return nil
}

func (f *File) loadLeafs(r *Reader) error {
	lump := &f.Header.Lumps[LumpLeafs]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		if f.Version == BSP2Version_BSP2 {
			count := int(lump.FileLength) / 32 // DL2Leaf is 32 bytes
			leafs := make([]DL2Leaf, count)
			for i := 0; i < count; i++ {
				offset := i * 32
				leafs[i] = DL2Leaf{
					Contents: int32(binary.LittleEndian.Uint32(data[offset:])),
					VisOfs:   int32(binary.LittleEndian.Uint32(data[offset+4:])),
					BoundsMin: [3]float32{
						Float32frombits(binary.LittleEndian.Uint32(data[offset+8:])),
						Float32frombits(binary.LittleEndian.Uint32(data[offset+12:])),
						Float32frombits(binary.LittleEndian.Uint32(data[offset+16:])),
					},
					BoundsMax: [3]float32{
						Float32frombits(binary.LittleEndian.Uint32(data[offset+20:])),
						Float32frombits(binary.LittleEndian.Uint32(data[offset+24:])),
						Float32frombits(binary.LittleEndian.Uint32(data[offset+28:])),
					},
				}
			}
			f.Leafs = leafs
		} else {
			count := int(lump.FileLength) / 28 // DL1Leaf is 28 bytes
			leafs := make([]DL1Leaf, count)
			for i := 0; i < count; i++ {
				offset := i * 28
				leafs[i] = DL1Leaf{
					Contents: int32(binary.LittleEndian.Uint32(data[offset:])),
					VisOfs:   int32(binary.LittleEndian.Uint32(data[offset+4:])),
					BoundsMin: [3]int16{
						int16(binary.LittleEndian.Uint16(data[offset+8:])),
						int16(binary.LittleEndian.Uint16(data[offset+10:])),
						int16(binary.LittleEndian.Uint16(data[offset+12:])),
					},
					BoundsMax: [3]int16{
						int16(binary.LittleEndian.Uint16(data[offset+14:])),
						int16(binary.LittleEndian.Uint16(data[offset+16:])),
						int16(binary.LittleEndian.Uint16(data[offset+18:])),
					},
					FirstMarkSurface: binary.LittleEndian.Uint32(data[offset+20:]),
					NumMarkSurfaces:  binary.LittleEndian.Uint32(data[offset+24:]),
				}
				copy(leafs[i].AmbientLevel[:], data[offset+28:offset+32])
			}
			f.Leafs = leafs
		}
	} else {
		count := int(lump.FileLength) / 28 // DSLeaf is 28 bytes
		leafs := make([]DSLeaf, count)
		for i := 0; i < count; i++ {
			offset := i * 28
			leafs[i] = DSLeaf{
				Contents: int32(binary.LittleEndian.Uint32(data[offset:])),
				VisOfs:   int32(binary.LittleEndian.Uint32(data[offset+4:])),
				BoundsMin: [3]int16{
					int16(binary.LittleEndian.Uint16(data[offset+8:])),
					int16(binary.LittleEndian.Uint16(data[offset+10:])),
					int16(binary.LittleEndian.Uint16(data[offset+12:])),
				},
				BoundsMax: [3]int16{
					int16(binary.LittleEndian.Uint16(data[offset+14:])),
					int16(binary.LittleEndian.Uint16(data[offset+16:])),
					int16(binary.LittleEndian.Uint16(data[offset+18:])),
				},
				FirstMarkSurface: binary.LittleEndian.Uint16(data[offset+20:]),
				NumMarkSurfaces:  binary.LittleEndian.Uint16(data[offset+22:]),
			}
			copy(leafs[i].AmbientLevel[:], data[offset+24:offset+28])
		}
		f.Leafs = leafs
	}
	return nil
}

func (f *File) loadMarkSurfaces(r *Reader) error {
	lump := &f.Header.Lumps[LumpMarksurfaces]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		count := int(lump.FileLength) / 4
		marks := make([]uint32, count)
		for i := 0; i < count; i++ {
			marks[i] = binary.LittleEndian.Uint32(data[i*4:])
		}
		f.MarkSurfaces = marks
	} else {
		count := int(lump.FileLength) / 2
		marks := make([]uint16, count)
		for i := 0; i < count; i++ {
			marks[i] = binary.LittleEndian.Uint16(data[i*2:])
		}
		f.MarkSurfaces = marks
	}
	return nil
}

func (f *File) loadEdges(r *Reader) error {
	lump := &f.Header.Lumps[LumpEdges]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		count := int(lump.FileLength) / 8 // DLEdge is 8 bytes
		edges := make([]DLEdge, count)
		for i := 0; i < count; i++ {
			offset := i * 8
			edges[i] = DLEdge{
				V: [2]uint32{
					binary.LittleEndian.Uint32(data[offset:]),
					binary.LittleEndian.Uint32(data[offset+4:]),
				},
			}
		}
		f.Edges = edges
	} else {
		count := int(lump.FileLength) / 4 // DSEdge is 4 bytes
		edges := make([]DSEdge, count)
		for i := 0; i < count; i++ {
			offset := i * 4
			edges[i] = DSEdge{
				V: [2]uint16{
					binary.LittleEndian.Uint16(data[offset:]),
					binary.LittleEndian.Uint16(data[offset+2:]),
				},
			}
		}
		f.Edges = edges
	}
	return nil
}

func (f *File) loadSurfedges(r *Reader) error {
	lump := &f.Header.Lumps[LumpSurfedges]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	count := int(lump.FileLength) / 4
	f.Surfedges = make([]int32, count)
	for i := 0; i < count; i++ {
		f.Surfedges[i] = int32(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return nil
}

func (f *File) loadModels(r *Reader) error {
	lump := &f.Header.Lumps[LumpModels]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	count := int(lump.FileLength) / 72 // DModel is 72 bytes
	f.Models = make([]DModel, count)

	for i := 0; i < count; i++ {
		offset := i * 72
		model := DModel{
			BoundsMin: [3]float32{
				Float32frombits(binary.LittleEndian.Uint32(data[offset:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+4:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+8:])),
			},
			BoundsMax: [3]float32{
				Float32frombits(binary.LittleEndian.Uint32(data[offset+12:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+16:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+20:])),
			},
			Origin: [3]float32{
				Float32frombits(binary.LittleEndian.Uint32(data[offset+24:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+28:])),
				Float32frombits(binary.LittleEndian.Uint32(data[offset+32:])),
			},
			VisLeafs:  int32(binary.LittleEndian.Uint32(data[offset+52:])),
			FirstFace: int32(binary.LittleEndian.Uint32(data[offset+56:])),
			NumFaces:  int32(binary.LittleEndian.Uint32(data[offset+60:])),
		}
		for j := 0; j < MaxMapHulls; j++ {
			model.HeadNode[j] = int32(binary.LittleEndian.Uint32(data[offset+36+j*4:]))
		}
		f.Models[i] = model
	}
	return nil
}

func (f *File) loadTextures(r *Reader) error {
	lump := &f.Header.Lumps[LumpTextures]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if len(data) >= 4 {
		f.NumTextures = int32(binary.LittleEndian.Uint32(data[0:4]))
	}
	f.TextureData = data
	return nil
}

// Float32frombits converts a uint32 to a float32 using IEEE 754 representation.
// This is used when reading binary floating-point values from BSP files.
func Float32frombits(b uint32) float32 {
	return math.Float32frombits(b)
}
