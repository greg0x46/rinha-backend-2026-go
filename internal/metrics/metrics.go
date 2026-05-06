package metrics

type Stage int

const (
	StageReadBody Stage = iota
	StageDecode
	StageVectorize
	StageScore
	StageWrite
	NumStages
)

var stageNames = [NumStages]string{
	"read_body",
	"decode",
	"vectorize",
	"score",
	"write",
}
