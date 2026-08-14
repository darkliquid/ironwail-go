package game

import (
	"encoding/binary"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

type camDebugFaceHit struct {
	faceIndex    int
	modelIndex   int
	entIndex     int
	distance     float32
	hitPos       types.Vec3
	planeIndex   int32
	planeSide    int32
	planeNormal  types.Vec3
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

func (g *Game) traceCrosshairFace(origin, forward types.Vec3) (*camDebugFaceHit, bool) {
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

	var vertBuf [64]types.Vec3

	traceModelFaces := func(modelIdx int, entIdx int, firstFace, numFaces int32, entOrigin types.Vec3) {
		if firstFace < 0 || firstFace+numFaces > int32(len(tree.Faces)) {
			return
		}

		localOrigin := origin.Sub(entOrigin)

		for i := firstFace; i < firstFace+numFaces; i++ {
			face := &tree.Faces[i]
			if face.PlaneNum < 0 || int(face.PlaneNum) >= len(tree.Planes) {
				continue
			}
			plane := &tree.Planes[face.PlaneNum]

			normal := plane.Normal
			dist := plane.Dist
			if face.Side != 0 {
				normal = types.Vec3{X: -normal.X, Y: -normal.Y, Z: -normal.Z}
				dist = -dist
			}

			denom := normal.X*forward.X + normal.Y*forward.Y + normal.Z*forward.Z
			if denom >= -1e-6 {
				continue
			}

			startDot := localOrigin.X*normal.X + localOrigin.Y*normal.Y + localOrigin.Z*normal.Z
			t := (dist - startDot) / denom
			if t < 0 || t >= bestDist {
				continue
			}

			localHit := types.Vec3{
				X: localOrigin.X + t*forward.X,
				Y: localOrigin.Y + t*forward.Y,
				Z: localOrigin.Z + t*forward.Z,
			}

			numVerts := int(face.NumEdges)
			if numVerts < 3 || face.FirstEdge < 0 || int(face.FirstEdge)+numVerts > len(tree.Surfedges) {
				continue
			}

			var verts []types.Vec3
			if numVerts <= len(vertBuf) {
				verts = vertBuf[:numVerts]
			} else {
				verts = make([]types.Vec3, numVerts)
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

				ex, ey, ez := v1.X-v0.X, v1.Y-v0.Y, v1.Z-v0.Z
				px, py, pz := localHit.X-v0.X, localHit.Y-v0.Y, localHit.Z-v0.Z

				cx := ey*pz - ez*py
				cy := ez*px - ex*pz
				cz := ex*py - ey*px

				dot := cx*normal.X + cy*normal.Y + cz*normal.Z
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

			worldHit := localHit.Add(entOrigin)

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
				hit.hitU = float64(localHit.X)*float64(ti.Vecs[0][0]) + float64(localHit.Y)*float64(ti.Vecs[0][1]) + float64(localHit.Z)*float64(ti.Vecs[0][2]) + float64(ti.Vecs[0][3])
				hit.hitV = float64(localHit.X)*float64(ti.Vecs[1][0]) + float64(localHit.Y)*float64(ti.Vecs[1][1]) + float64(localHit.Z)*float64(ti.Vecs[1][2]) + float64(ti.Vecs[1][3])

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
		traceModelFaces(0, 0, worldModel.FirstFace, worldModel.NumFaces, types.Vec3{})
	}

	if g.Server != nil && g.Server.Edicts != nil {
		for edictIdx, ent := range g.Server.Edicts {
			if ent == nil || ent.Free {
				continue
			}
			modelIdx := int(ent.ModelIndex(g.Server))
			if modelIdx > 0 && modelIdx < len(tree.Models) {
				submodel := &tree.Models[modelIdx]
				traceModelFaces(modelIdx, edictIdx, submodel.FirstFace, submodel.NumFaces, ent.Origin(g.Server))
			}
		}
	}

	if !found {
		return nil, false
	}
	return &bestHit, true
}
