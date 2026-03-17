package server

import "github.com/ironwail/ironwail-go/internal/model"

// CollisionModel abstracts the collision-relevant aspects of a BSP world model.
// The server only needs collision data from the world model — it never accesses
// rendering data. This interface decouples the server from the concrete
// model.Model type, improving testability and making the dependency explicit.
type CollisionModel interface {
	// ModelType returns the model type (brush, alias, sprite).
	ModelType() int
	// NumHulls returns how many collision hulls this model provides.
	NumHulls() int
	// Hull returns the collision hull at the given index.
	Hull(index int) model.Hull
	// CollisionClipNodes returns the BSP clip nodes for collision detection.
	CollisionClipNodes() []model.MClipNode
	// CollisionPlanes returns the BSP planes used by clip nodes.
	CollisionPlanes() []model.MPlane
	// IsClipBox returns whether this model uses simplified box clipping.
	IsClipBox() bool
	// CollisionClipMins returns the minimum bounds of the clip box.
	CollisionClipMins() [3]float32
	// CollisionClipMaxs returns the maximum bounds of the clip box.
	CollisionClipMaxs() [3]float32
}
