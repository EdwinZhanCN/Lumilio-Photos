package testfixture

import (
	"io/fs"
	"server/internal/storage"
)

// Observer records every read and can inject facts without depending on the
// host OS, privileges, mounts, or path-name heuristics. It has no write methods.
// Tests configure it before use; it is not a concurrent event recorder.
type Observer struct {
	Base             storage.StorageObserver
	Calls            []string
	StatFunc         func(string) (fs.FileInfo, error)
	ReadFileFunc     func(string) ([]byte, error)
	ReadDirFunc      func(string) ([]fs.DirEntry, error)
	EvalSymlinksFunc func(string) (string, error)
	StatFSFunc       func(string) (storage.VolumeFacts, error)
	MountInfoFunc    func(string) (storage.MountFacts, error)
	PlaceholderFunc  func(string) (bool, error)
}

func (o *Observer) base() storage.StorageObserver {
	if o.Base != nil {
		return o.Base
	}
	return storage.OSStorageObserver{}
}
func (o *Observer) Stat(p string) (fs.FileInfo, error) {
	o.Calls = append(o.Calls, "stat:"+p)
	if o.StatFunc != nil {
		return o.StatFunc(p)
	}
	return o.base().Stat(p)
}
func (o *Observer) ReadFile(p string) ([]byte, error) {
	o.Calls = append(o.Calls, "read:"+p)
	if o.ReadFileFunc != nil {
		return o.ReadFileFunc(p)
	}
	return o.base().ReadFile(p)
}
func (o *Observer) ReadDir(p string) ([]fs.DirEntry, error) {
	o.Calls = append(o.Calls, "readdir:"+p)
	if o.ReadDirFunc != nil {
		return o.ReadDirFunc(p)
	}
	return o.base().ReadDir(p)
}
func (o *Observer) EvalSymlinks(p string) (string, error) {
	o.Calls = append(o.Calls, "resolve:"+p)
	if o.EvalSymlinksFunc != nil {
		return o.EvalSymlinksFunc(p)
	}
	return o.base().EvalSymlinks(p)
}
func (o *Observer) StatFS(p string) (storage.VolumeFacts, error) {
	o.Calls = append(o.Calls, "statfs:"+p)
	if o.StatFSFunc != nil {
		return o.StatFSFunc(p)
	}
	return o.base().StatFS(p)
}
func (o *Observer) MountInfo(p string) (storage.MountFacts, error) {
	o.Calls = append(o.Calls, "mount:"+p)
	if o.MountInfoFunc != nil {
		return o.MountInfoFunc(p)
	}
	return o.base().MountInfo(p)
}
func (o *Observer) Placeholder(p string) (bool, error) {
	o.Calls = append(o.Calls, "attributes:"+p)
	if o.PlaceholderFunc != nil {
		return o.PlaceholderFunc(p)
	}
	return o.base().Placeholder(p)
}

var _ storage.StorageObserver = (*Observer)(nil)
