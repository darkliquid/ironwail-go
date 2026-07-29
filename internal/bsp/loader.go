package bsp

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/darkliquid/ironwail-go/internal/engine/arena"
)

func validateLumpRecordSize(context string, data []byte, recordSize int) error {
	if len(data)%recordSize != 0 {
		return fmt.Errorf("%s: funny lump size %d", context, len(data))
	}
	return nil
}

// Load reads and parses a complete BSP file from the reader.
// It returns a File struct containing all the parsed data.
func Load(r io.ReadSeeker) (*File, error) {
	return LoadWithArena(r, nil)
}

// LoadWithArena reads and parses a complete BSP file using the provided arena allocator.
func LoadWithArena(r io.ReadSeeker, ar *arena.Arena) (*File, error) {
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
	if err := file.loadPlanes(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load planes: %w", err)
	}
	if err := file.loadVertexes(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load vertexes: %w", err)
	}
	if err := file.loadVisibility(reader); err != nil {
		return nil, fmt.Errorf("failed to load visibility: %w", err)
	}
	if err := file.loadNodes(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load nodes: %w", err)
	}
	if err := file.loadTexinfo(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load texinfo: %w", err)
	}
	if err := file.loadFaces(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load faces: %w", err)
	}
	if err := file.loadLighting(reader); err != nil {
		return nil, fmt.Errorf("failed to load lighting: %w", err)
	}
	if err := file.loadClipnodes(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load clipnodes: %w", err)
	}
	if err := file.loadLeafs(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load leafs: %w", err)
	}
	if err := file.loadMarkSurfaces(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load marksurfaces: %w", err)
	}
	if err := file.loadEdges(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load edges: %w", err)
	}
	if err := file.loadSurfedges(reader, ar); err != nil {
		return nil, fmt.Errorf("failed to load surfedges: %w", err)
	}
	if err := file.loadModels(reader, ar); err != nil {
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

func (f *File) loadPlanes(r *Reader, ar *arena.Arena) error {
	lump := &f.Header.Lumps[LumpPlanes]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}
	if err := validateLumpRecordSize("load planes", data, dPlaneSize); err != nil {
		return err
	}

	count := len(data) / dPlaneSize
	f.Planes = arena.Alloc[DPlane](ar, count)

	for i := 0; i < count; i++ {
		offset := i * dPlaneSize
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

func (f *File) loadVertexes(r *Reader, ar *arena.Arena) error {
	lump := &f.Header.Lumps[LumpVertexes]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}
	if err := validateLumpRecordSize("load vertexes", data, dVertexSize); err != nil {
		return err
	}

	count := len(data) / dVertexSize
	f.Vertexes = arena.Alloc[DVertex](ar, count)

	for i := 0; i < count; i++ {
		offset := i * dVertexSize
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

func (f *File) loadNodes(r *Reader, ar *arena.Arena) error {
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
			if len(data)%dl2NodeSize != 0 {
				return fmt.Errorf("load nodes: funny lump size %d", len(data))
			}
			count := int(lump.FileLength) / dl2NodeSize
			nodes := arena.Alloc[DL2Node](ar, count)
			for i := 0; i < count; i++ {
				offset := i * dl2NodeSize
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
					FirstFace: binary.LittleEndian.Uint32(data[offset+36:]),
					NumFaces:  binary.LittleEndian.Uint32(data[offset+40:]),
				}
			}
			f.Nodes = nodes
		} else {
			if len(data)%dl1NodeSize != 0 {
				return fmt.Errorf("load nodes: funny lump size %d", len(data))
			}
			count := int(lump.FileLength) / dl1NodeSize
			nodes := arena.Alloc[DL1Node](ar, count)
			for i := 0; i < count; i++ {
				offset := i * dl1NodeSize
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
					FirstFace: binary.LittleEndian.Uint32(data[offset+24:]),
					NumFaces:  binary.LittleEndian.Uint32(data[offset+28:]),
				}
			}
			f.Nodes = nodes
		}
	} else {
		if len(data)%dsNodeSize != 0 {
			return fmt.Errorf("load nodes: funny lump size %d", len(data))
		}
		count := int(lump.FileLength) / dsNodeSize
		nodes := arena.Alloc[DSNode](ar, count)
		for i := 0; i < count; i++ {
			offset := i * dsNodeSize
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

func (f *File) loadTexinfo(r *Reader, ar *arena.Arena) error {
	lump := &f.Header.Lumps[LumpTexinfo]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}
	if err := validateLumpRecordSize("load texinfo", data, 40); err != nil {
		return err
	}

	count := len(data) / 40
	f.Texinfo = arena.Alloc[Texinfo](ar, count)

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

func (f *File) loadFaces(r *Reader, ar *arena.Arena) error {
	lump := &f.Header.Lumps[LumpFaces]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		if err := validateLumpRecordSize("load faces", data, dlFaceSize); err != nil {
			return err
		}
		count := len(data) / dlFaceSize
		faces := arena.Alloc[DLFace](ar, count)
		for i := 0; i < count; i++ {
			offset := i * dlFaceSize
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
		if err := validateLumpRecordSize("load faces", data, dsFaceSize); err != nil {
			return err
		}
		count := len(data) / dsFaceSize
		faces := arena.Alloc[DSFace](ar, count)
		for i := 0; i < count; i++ {
			offset := i * dsFaceSize
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

func (f *File) loadClipnodes(r *Reader, ar *arena.Arena) error {
	lump := &f.Header.Lumps[LumpClipnodes]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		if err := validateLumpRecordSize("load clipnodes", data, 12); err != nil {
			return err
		}
		count := len(data) / 12
		clipnodes := arena.Alloc[DLClipNode](ar, count)
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
		if err := validateLumpRecordSize("load clipnodes", data, 8); err != nil {
			return err
		}
		count := len(data) / 8
		clipnodes := arena.Alloc[DSClipNode](ar, count)
		for i := 0; i < count; i++ {
			offset := i * 8
			child0 := standardClipnodeChild(binary.LittleEndian.Uint16(data[offset+4:]), count)
			child1 := standardClipnodeChild(binary.LittleEndian.Uint16(data[offset+6:]), count)
			clipnodes[i] = DSClipNode{
				PlaneNum: int32(binary.LittleEndian.Uint32(data[offset:])),
				Children: [2]int32{child0, child1},
			}
		}
		f.Clipnodes = clipnodes
	}
	return nil
}

func (f *File) loadLeafs(r *Reader, ar *arena.Arena) error {
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
			if len(data)%dl2LeafSize != 0 {
				return fmt.Errorf("load leafs: funny lump size %d", len(data))
			}
			count := int(lump.FileLength) / dl2LeafSize
			leafs := arena.Alloc[DL2Leaf](ar, count)
			for i := 0; i < count; i++ {
				offset := i * dl2LeafSize
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
					FirstMarkSurface: binary.LittleEndian.Uint32(data[offset+32:]),
					NumMarkSurfaces:  binary.LittleEndian.Uint32(data[offset+36:]),
				}
				copy(leafs[i].AmbientLevel[:], data[offset+40:offset+44])
			}
			f.Leafs = leafs
		} else {
			if len(data)%dl1LeafSize != 0 {
				return fmt.Errorf("load leafs: funny lump size %d", len(data))
			}
			count := int(lump.FileLength) / dl1LeafSize
			leafs := arena.Alloc[DL1Leaf](ar, count)
			for i := 0; i < count; i++ {
				offset := i * dl1LeafSize
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
		if err := validateLumpRecordSize("load leafs", data, dsLeafSize); err != nil {
			return err
		}
		count := len(data) / dsLeafSize
		leafs := arena.Alloc[DSLeaf](ar, count)
		for i := 0; i < count; i++ {
			offset := i * dsLeafSize
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

func (f *File) loadMarkSurfaces(r *Reader, ar *arena.Arena) error {
	lump := &f.Header.Lumps[LumpMarksurfaces]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		if err := validateLumpRecordSize("load marksurfaces", data, uint32Size); err != nil {
			return err
		}
		count := len(data) / uint32Size
		marks := arena.Alloc[uint32](ar, count)
		for i := 0; i < count; i++ {
			marks[i] = binary.LittleEndian.Uint32(data[i*4:])
		}
		f.MarkSurfaces = marks
	} else {
		if err := validateLumpRecordSize("load marksurfaces", data, uint16Size); err != nil {
			return err
		}
		count := len(data) / uint16Size
		marks := arena.Alloc[uint16](ar, count)
		for i := 0; i < count; i++ {
			marks[i] = binary.LittleEndian.Uint16(data[i*2:])
		}
		f.MarkSurfaces = marks
	}
	return nil
}

func (f *File) loadEdges(r *Reader, ar *arena.Arena) error {
	lump := &f.Header.Lumps[LumpEdges]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if f.IsBSP2 {
		if err := validateLumpRecordSize("load edges", data, dlEdgeSize); err != nil {
			return err
		}
		count := len(data) / dlEdgeSize
		edges := arena.Alloc[DLEdge](ar, count)
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
		if err := validateLumpRecordSize("load edges", data, dsEdgeSize); err != nil {
			return err
		}
		count := len(data) / dsEdgeSize
		edges := arena.Alloc[DSEdge](ar, count)
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

func (f *File) loadSurfedges(r *Reader, ar *arena.Arena) error {
	lump := &f.Header.Lumps[LumpSurfedges]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}
	if err := validateLumpRecordSize("load surfedges", data, int32Size); err != nil {
		return err
	}

	count := len(data) / int32Size
	f.Surfedges = arena.Alloc[int32](ar, count)
	for i := 0; i < count; i++ {
		f.Surfedges[i] = int32(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return nil
}

func standardClipnodeChild(raw uint16, clipnodeCount int) int32 {
	child := int(raw)
	if child >= clipnodeCount {
		child -= 65536
	}
	return int32(child)
}

func (f *File) loadModels(r *Reader, ar *arena.Arena) error {
	lump := &f.Header.Lumps[LumpModels]
	if lump.FileLength == 0 {
		return nil
	}

	data, err := r.ReadLump(lump)
	if err != nil {
		return err
	}

	if len(data)%dModelSize != 0 {
		return fmt.Errorf("load models: funny lump size %d", len(data))
	}
	count := int(lump.FileLength) / dModelSize
	f.Models = arena.Alloc[DModel](ar, count)

	for i := 0; i < count; i++ {
		offset := i * dModelSize
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
