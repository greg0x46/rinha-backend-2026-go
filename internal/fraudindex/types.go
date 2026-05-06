package fraudindex

type Vector [14]float32

type Label uint8

const (
	LabelLegit Label = iota
	LabelFraud
)

type Reference struct {
	Vector Vector
	Label  Label
}
