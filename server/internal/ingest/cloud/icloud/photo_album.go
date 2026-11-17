package icloud

import (
	"fmt"
	"sync"
)

const (
	AlbumNameAll             = "All Photos"
	AlbumNameTimeLapse       = "Time-lapse"
	AlbumNameVideos          = "Videos"
	AlbumNameSloMo           = "Slo-mo"
	AlbumNameBursts          = "Bursts"
	AlbumNameFavorites       = "Favorites"
	AlbumNamePanoramas       = "Panoramas"
	AlbumNameScreenshots     = "Screenshots"
	AlbumNameLive            = "Live"
	AlbumNameRecentlyDeleted = "Recently Deleted"
	AlbumNameHidden          = "Hidden"
)

type PhotoAlbum struct {
	// service
	service *PhotoService

	// attr
	Name        string
	ListType    string
	ObjType     string
	Direction   string
	QueryFilter []*folderMetaDataQueryFilter

	// cache
	_size *int64
	lock  *sync.Mutex
}

func (r *PhotoService) newPhotoAlbum(name, listType, objType, direction string, queryFilter []*folderMetaDataQueryFilter) *PhotoAlbum {
	return &PhotoAlbum{
		service: r,

		Name:        name,
		ListType:    listType,
		ObjType:     objType,
		Direction:   direction,
		QueryFilter: queryFilter,

		_size: nil,
		lock:  new(sync.Mutex),
	}
}

func (r *PhotoService) GetAlbum(albumName string) (*PhotoAlbum, error) {
	albums, err := r.Albums()
	if err != nil {
		return nil, err
	}

	album := albums[AlbumNameAll]
	if albumName != "" {
		var ok bool
		album, ok = albums[albumName]
		if !ok {
			return nil, fmt.Errorf("album %s not found", albumName)
		}
	}
	return album, nil
}

func (r *PhotoService) Albums() (map[string]*PhotoAlbum, error) {
	r.lock.Lock()
	albumIsNil := len(r._albums) == 0
	r.lock.Unlock()

	if !albumIsNil {
		return r._albums, nil
	}

	tmp := map[string]*PhotoAlbum{}

	// Only expose built-in albums; user-created folders are not supported.
	for name, props := range icloudPhotoFolderMeta {
		tmp[name] = r.newPhotoAlbum(name, props.ListType, props.ObjType, props.Direction, props.QueryFilter)
	}

	r.lock.Lock()
	r._albums = tmp
	r.lock.Unlock()

	return r._albums, nil
}

// folderTypeValue is a generic typed value used in CloudKit query filters.
type folderTypeValue struct {
	Value any    `json:"value"`
	Type  string `json:"type"`
}

var icloudPhotoFolderMeta = map[string]*folderMetaData{
	"All Photos": {
		ObjType:   "CPLAssetByAddedDate",
		ListType:  "CPLAssetAndMasterByAddedDate",
		Direction: "ASCENDING",
	},
	"Time-lapse": {
		ObjType:   "CPLAssetInSmartAlbumByAssetDate:Timelapse",
		ListType:  "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction: "ASCENDING",
		QueryFilter: []*folderMetaDataQueryFilter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: &folderTypeValue{Type: "STRING", Value: "TIMELAPSE"},
			},
		},
	},
	"Videos": {
		ObjType:   "CPLAssetInSmartAlbumByAssetDate:Video",
		ListType:  "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction: "ASCENDING",
		QueryFilter: []*folderMetaDataQueryFilter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: &folderTypeValue{Type: "STRING", Value: "VIDEO"},
			},
		},
	},
	"Slo-mo": {
		ObjType:   "CPLAssetInSmartAlbumByAssetDate:Slomo",
		ListType:  "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction: "ASCENDING",
		QueryFilter: []*folderMetaDataQueryFilter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: &folderTypeValue{Type: "STRING", Value: "SLOMO"},
			},
		},
	},
	"Bursts": {
		ObjType:   "CPLAssetBurstStackAssetByAssetDate",
		ListType:  "CPLBurstStackAssetAndMasterByAssetDate",
		Direction: "ASCENDING",
	},
	"Favorites": {
		ObjType:   "CPLAssetInSmartAlbumByAssetDate:Favorite",
		ListType:  "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction: "ASCENDING",
		QueryFilter: []*folderMetaDataQueryFilter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: &folderTypeValue{Type: "STRING", Value: "FAVORITE"},
			},
		},
	},
	"Panoramas": {
		ObjType:   "CPLAssetInSmartAlbumByAssetDate:Panorama",
		ListType:  "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction: "ASCENDING",
		QueryFilter: []*folderMetaDataQueryFilter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: &folderTypeValue{Type: "STRING", Value: "PANORAMA"},
			},
		},
	},
	"Screenshots": {
		ObjType:   "CPLAssetInSmartAlbumByAssetDate:Screenshot",
		ListType:  "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction: "ASCENDING",
		QueryFilter: []*folderMetaDataQueryFilter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: &folderTypeValue{Type: "STRING", Value: "SCREENSHOT"},
			},
		},
	},
	"Live": {
		ObjType:   "CPLAssetInSmartAlbumByAssetDate:Live",
		ListType:  "CPLAssetAndMasterInSmartAlbumByAssetDate",
		Direction: "ASCENDING",
		QueryFilter: []*folderMetaDataQueryFilter{
			{
				FieldName:  "smartAlbum",
				Comparator: "EQUALS",
				FieldValue: &folderTypeValue{Type: "STRING", Value: "LIVE"},
			},
		},
	},
	"Recently Deleted": {
		ObjType:   "CPLAssetDeletedByExpungedDate",
		ListType:  "CPLAssetAndMasterDeletedByExpungedDate",
		Direction: "ASCENDING",
	},
	"Hidden": {
		ObjType:   "CPLAssetHiddenByAssetDate",
		ListType:  "CPLAssetAndMasterHiddenByAssetDate",
		Direction: "ASCENDING",
	},
}

type folderMetaData struct {
	ListType    string                       `json:"list_type"`
	ObjType     string                       `json:"obj_type"`
	Direction   string                       `json:"direction"`
	QueryFilter []*folderMetaDataQueryFilter `json:"query_filter"`
	PageSize    int                          `json:"page_size"`
}

type folderMetaDataQueryFilter struct {
	FieldName  string           `json:"fieldName"`
	Comparator string           `json:"comparator"`
	FieldValue *folderTypeValue `json:"fieldValue"`
}
