package progs

type Vec3 [3]float32

func (v Vec3) Add(o Vec3) Vec3      { return Vec3{} }
func (v Vec3) Sub(o Vec3) Vec3      { return Vec3{} }
func (v Vec3) Scale(s float32) Vec3 { return Vec3{} }
func (v Vec3) Dot(o Vec3) float32   { return 0 }

func Compose(ax, ay, az, bx, by, bz, s float32) float32 {
	a := Vec3{ax, ay, az}
	b := Vec3{bx, by, bz}
	return a.Add(b).Sub(b).Scale(s).Dot(b)
}
