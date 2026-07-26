package repo

const (
	AlbumTypeDefault = "default"
	AlbumTypeBio     = "bio"
)

type StackRelation string

const (
	StackRelationRawOriginal    StackRelation = "raw_original"
	StackRelationJpegOriginal   StackRelation = "jpeg_original"
	StackRelationEditedVersion  StackRelation = "edited_version"
	StackRelationAlternative    StackRelation = "alternative"
	StackRelationLivePhotoStill StackRelation = "live_photo_still"
	StackRelationLivePhotoVideo StackRelation = "live_photo_video"
)
