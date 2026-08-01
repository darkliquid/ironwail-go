package game

import (
	"encoding/binary"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

type camDebugFaceHit struct {
	faceIndex    int
	modelIndex   int
	entIndex     int
	distance     float32
	hitPos       [3]float32
	planeIndex   int32
	planeSide    int32
	planeNormal  [3]float32
	planeDist    float32
	texinfoIdx   int32
	texName      string
	texWidth     int
	texHeight    int
	texType      model.TextureType
	texFlags     int32
	derivedFlags int32
	numEdges     int32
	firstEdge    int32
	lightOfs     int32
	styles       [4]byte
	hitU, hitV   float64
}

func (g *Game) traceCrosshairFace(origin, forward [3]float32) (*camDebugFaceHit, bool) {
	if g.Server == nil || g.Server.WorldTree == nil {
		return nil, false
	}
	tree := g.Server.WorldTree
	if len(tree.Faces) == 0 || len(tree.Planes) == 0 {
		return nil, false
	}

	bestDist := float32(16384.0)
	var bestHit camDebugFaceHit
	found := false

	var vertBuf [64][3]float32

	traceModelFaces := func(modelIdx int, entIdx int, firstFace, numFaces int32, entOrigin [3]float32) {
		if firstFace < 0 || firstFace+numFaces > int32(len(tree.Faces)) {
			return
		}

		localOrigin := [3]float32{
			origin[0] - entOrigin[0],
			origin[1] - entOrigin[1],
			origin[2] - entOrigin[2],
		}

		for i := firstFace; i < firstFace+numFaces; i++ {
			face := &tree.Faces[i]
			if face.PlaneNum < 0 || int(face.PlaneNum) >= len(tree.Planes) {
				continue
			}
			plane := &tree.Planes[face.PlaneNum]

			normal := plane.Normal
			dist := plane.Dist
			if face.Side != 0 {
				normal = [3]float32{-normal[0], -normal[1], -normal[2]}
				dist = -dist
			}

			denom := normal[0]*forward[0] + normal[1]*forward[1] + normal[2]*forward[2]
			if denom >= -1e-6 {
				continue
			}

			startDot := localOrigin[0]*normal[0] + localOrigin[1]*normal[1] + localOrigin[2]*normal[2]
			t := (dist - startDot) / denom
			if t < 0 || t >= bestDist {
				continue
			}

			localHit := [3]float32{
				localOrigin[0] + t*forward[0],
				localOrigin[1] + t*forward[1],
				localOrigin[2] + t*forward[2],
			}

			numVerts := int(face.NumEdges)
			if numVerts < 3 || face.FirstEdge < 0 || int(face.FirstEdge)+numVerts > len(tree.Surfedges) {
				continue
			}

			var verts [][3]float32
			if numVerts <= len(vertBuf) {
				verts = vertBuf[:numVerts]
			} else {
				verts = make([][3]float32, numVerts)
			}

			validVerts := true
			for j := 0; j < numVerts; j++ {
				surfEdge := tree.Surfedges[int(face.FirstEdge)+j]
				var vi uint32
				if surfEdge >= 0 {
					if int(surfEdge) >= len(tree.Edges) {
						validVerts = false
						break
					}
					vi = tree.Edges[surfEdge].V[0]
				} else {
					edgeIdx := -surfEdge
					if int(edgeIdx) >= len(tree.Edges) {
						validVerts = false
						break
					}
					vi = tree.Edges[edgeIdx].V[1]
				}

				if int(vi) >= len(tree.Vertexes) {
					validVerts = false
					break
				}
				verts[j] = tree.Vertexes[vi].Point
			}

			if !validVerts {
				continue
			}

			posCount := 0
			negCount := 0
			inside := true

			for j := 0; j < numVerts; j++ {
				v0 := verts[j]
				v1 := verts[(j+1)%numVerts]

				ex, ey, ez := v1[0]-v0[0], v1[1]-v0[1], v1[2]-v0[2]
				px, py, pz := localHit[0]-v0[0], localHit[1]-v0[1], localHit[2]-v0[2]

				cx := ey*pz - ez*py
				cy := ez*px - ex*pz
				cz := ex*py - ey*px

				dot := cx*normal[0] + cy*normal[1] + cz*normal[2]
				if dot > 0.05 {
					posCount++
				} else if dot < -0.05 {
					negCount++
				}

				if posCount > 0 && negCount > 0 {
					inside = false
					break
				}
			}

			if !inside {
				continue
			}

			bestDist = t
			found = true

			worldHit := [3]float32{
				localHit[0] + entOrigin[0],
				localHit[1] + entOrigin[1],
				localHit[2] + entOrigin[2],
			}

			hit := camDebugFaceHit{
				faceIndex:   int(i),
				modelIndex:  modelIdx,
				entIndex:    entIdx,
				distance:    t,
				hitPos:      worldHit,
				planeIndex:  face.PlaneNum,
				planeSide:   face.Side,
				planeNormal: normal,
				planeDist:   dist,
				texinfoIdx:  face.Texinfo,
				numEdges:    face.NumEdges,
				firstEdge:   face.FirstEdge,
				lightOfs:    face.LightOfs,
				styles:      face.Styles,
			}

			if int(face.Texinfo) >= 0 && int(face.Texinfo) < len(tree.Texinfo) {
				ti := &tree.Texinfo[face.Texinfo]
				hit.texFlags = ti.Flags
				hit.hitU = float64(localHit[0])*float64(ti.Vecs[0][0]) + float64(localHit[1])*float64(ti.Vecs[0][1]) + float64(localHit[2])*float64(ti.Vecs[0][2]) + float64(ti.Vecs[0][3])
				hit.hitV = float64(localHit[0])*float64(ti.Vecs[1][0]) + float64(localHit[1])*float64(ti.Vecs[1][1]) + float64(localHit[2])*float64(ti.Vecs[1][2]) + float64(ti.Vecs[1][3])

				if len(tree.TextureData) >= 4 {
					textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
					if int(ti.Miptex) >= 0 && int(ti.Miptex) < textureCount {
						ofsPos := 4 + int(ti.Miptex)*4
						if ofsPos+4 <= len(tree.TextureData) {
							offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[ofsPos : ofsPos+4])))
							if offset > 0 && offset < len(tree.TextureData) {
								mt, err := image.ParseMipTex(tree.TextureData[offset:])
								if err == nil {
									hit.texName = mt.Name
									hit.texWidth = int(mt.Width)
									hit.texHeight = int(mt.Height)
									hit.texType = worldimpl.ClassifyTextureName(mt.Name)
									hit.derivedFlags = worldimpl.DeriveFaceFlags(hit.texType, hit.texFlags)
								}
							}
						}
					}
				}
			}

			bestHit = hit
		}
	}

	if len(tree.Models) > 0 {
		worldModel := &tree.Models[0]
		traceModelFaces(0, 0, worldModel.FirstFace, worldModel.NumFaces, [3]float32{0, 0, 0})
	}

	if g.Server != nil && g.Server.Edicts != nil {
		for edictIdx, ent := range g.Server.Edicts {
			if ent == nil || ent.Free {
				continue
			}
			modelIdx := int(ent.Vars.ModelIndex)
			if modelIdx > 0 && modelIdx < len(tree.Models) {
				submodel := &tree.Models[modelIdx]
				traceModelFaces(modelIdx, edictIdx, submodel.FirstFace, submodel.NumFaces, ent.Vars.Origin)
			}
		}
	}

	if !found {
		return nil, false
	}
	return &bestHit, true
}
