package constant

type TaskPlatform string

const (
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney              = "mj"
)

const (
	SunoActionMusic  = "MUSIC"
	SunoActionLyrics = "LYRICS"

	TaskActionGenerate          = "generate"
	TaskActionTextGenerate      = "textGenerate"
	TaskActionFirstTailGenerate = "firstTailGenerate"
	TaskActionReferenceGenerate = "referenceGenerate"
	TaskActionRemix             = "remixGenerate"
	TaskActionMiniMaxH3Generate = "minimaxH3Generate"
)

// MiniMax-H3 model identifiers used across the gateway.
const (
	ModelMiniMaxH3       = "MiniMax-H3"
	ModelMiniMaxH3Ref2va = "MiniMax-H3-Ref2va"
)

// MiniMaxH3Models tracks every model that should be handled by the MiniMax-H3 pipeline.
var MiniMaxH3Models = map[string]bool{
	ModelMiniMaxH3:       true,
	ModelMiniMaxH3Ref2va: true,
}

// IsMiniMaxH3Model reports whether the model belongs to the MiniMax-H3 family.
func IsMiniMaxH3Model(modelName string) bool {
	return MiniMaxH3Models[modelName]
}

var SunoModel2Action = map[string]string{
	"suno_music":  SunoActionMusic,
	"suno_lyrics": SunoActionLyrics,
}
