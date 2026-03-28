package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

func main() {
	engine.Dprint("main function\n")

	// these are just commands the the prog compiler to copy these files
	engine.PrecacheFile("progs.dat")
	engine.PrecacheFile("gfx.wad")
	engine.PrecacheFile("quake.rc")
	engine.PrecacheFile("default.cfg")

	engine.PrecacheFile("end1.bin")
	engine.PrecacheFile("end2.bin")

	engine.PrecacheFile("demo1.dem")
	engine.PrecacheFile("demo2.dem")
	engine.PrecacheFile("demo3.dem")
}

func StartFrame() {
	Framecount = Framecount + 1
}

func worldspawn() {
	World = Self
	engine.PrecacheModel(Self.Model)
}

func InitBodyQueue() {
	body1 := engine.Spawn()
	body2 := engine.Spawn()
	body3 := engine.Spawn()
	body4 := engine.Spawn()

	BodyqueueHead = body1
	body1.Owner = body2
	body2.Owner = body3
	body3.Owner = body4
	body4.Owner = body1
}

func CopyToBodyQueue(ent *quake.Entity) {
	BodyqueueHead.Angles = ent.Angles
	BodyqueueHead.Model = ent.Model
	BodyqueueHead.ModelIndex = ent.ModelIndex
	BodyqueueHead.Frame = ent.Frame
	BodyqueueHead.ColorMap = ent.ColorMap
	BodyqueueHead.MoveType = ent.MoveType
	BodyqueueHead.Velocity = ent.Velocity
	BodyqueueHead.Flags = 0
	engine.SetOrigin(BodyqueueHead, ent.Origin)
	engine.SetSize(BodyqueueHead, ent.Mins, ent.Maxs)
	BodyqueueHead = BodyqueueHead.Owner
}
