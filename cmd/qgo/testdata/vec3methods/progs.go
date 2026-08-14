package progs

type Vec3 [3]float32

func (v Vec3) Add(o Vec3) Vec3                 { return Vec3{} }
func (v Vec3) Sub(o Vec3) Vec3                 { return Vec3{} }
func (v Vec3) Mul(s float32) Vec3              { return Vec3{} }
func (v Vec3) Scale(s float32) Vec3            { return Vec3{} }
func (v Vec3) Div(s float32) Vec3              { return Vec3{} }
func (v Vec3) Dot(o Vec3) float32              { return 0 }
func (v Vec3) Neg() Vec3                       { return Vec3{} }
func (v Vec3) Negate() Vec3                    { return Vec3{} }
func (v Vec3) LenSq() float32                  { return 0 }
func (v Vec3) LengthSq() float32               { return 0 }
func (v Vec3) Equals(o Vec3) float32           { return 0 }
func (v Vec3) MA(scale float32, b Vec3) Vec3   { return Vec3{} }
func (v Vec3) MultiplyAdd(s float32, b Vec3) Vec3 { return Vec3{} }

func Compose(ax, ay, az, bx, by, bz, s float32) float32 {
	a := Vec3{ax, ay, az}
	b := Vec3{bx, by, bz}
	return a.Add(b).Sub(b).Scale(s).Dot(b)
}

func ComposeExtended(ax, ay, az, bx, by, bz, s float32) float32 {
	a := Vec3{ax, ay, az}
	b := Vec3{bx, by, bz}
	c := a.Mul(s).Div(s).Neg().Negate().MA(s, b).MultiplyAdd(s, b)
	val := c.LenSq() + c.LengthSq() + c.Equals(b)
	return c.Dot(b) + val
}



