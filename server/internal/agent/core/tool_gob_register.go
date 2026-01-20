package core

import (
	"encoding/gob"
	"server/internal/api/dto"
	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgtype"
)

// !!! 关键：必须在 init 中注册所有可能存入 Reference 的结构体 !!!
// 否则从数据库 Resume 时，Gob 无法反序列化 interface{}
func init() {
	// For ReferenceManager
	gob.Register(&ReferenceMeta{})

	// For tool outputs that are stored in ReferenceManager
	gob.Register(repo.Asset{})
	gob.Register([]repo.Asset{})
	gob.Register(&dto.BulkLikeUpdateDTO{})

	// For type converters
	gob.Register([]string{})

	// For tool interrupts
	gob.Register(&FilterConfirmationInfo{})
	gob.Register(&FilterInterruptState{})

	// For pgtype used in repo.Asset
	gob.Register(pgtype.UUID{})
	gob.Register(pgtype.Timestamp{})
	gob.Register(pgtype.Text{})
	gob.Register(pgtype.Int8{})
	gob.Register(pgtype.Int4{})
	gob.Register(pgtype.Bool{})
}
