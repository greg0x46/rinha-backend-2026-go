package fraudindex

type Vector [14]float32

type QuantizedVector [14]int16

type Label uint8

const (
	LabelLegit Label = iota
	LabelFraud
)

type Reference struct {
	Vector Vector
	Label  Label
}

type QuantizedIndex struct {
	Vectors []QuantizedVector
	Labels  []Label
}
