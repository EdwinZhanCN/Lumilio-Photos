package processors

import (
	"context"
	"fmt"
	"io"
	"path"

	"server/internal/db/repo"
	"server/internal/storage"
)

func (ap *AssetProcessor) saveThumbnail(ctx context.Context, files *storage.RepositoryFS, reader io.Reader, asset *repo.Asset, size string) error {
	privatePath, err := derivedPath("thumbnails", size, fmt.Sprintf("%s_%s.webp", asset.ContentHash, size))
	if err != nil {
		return err
	}
	if err := writeDerived(files, privatePath, reader); err != nil {
		return err
	}
	if _, err := ap.assetService.CreateThumbnail(ctx, asset.AssetID, size, privatePath.String()); err != nil {
		return fmt.Errorf("record thumbnail: %w", err)
	}
	return nil
}

func saveVideoVersion(files *storage.RepositoryFS, reader io.Reader, asset *repo.Asset, version string) error {
	privatePath, err := derivedPath("videos", version, fmt.Sprintf("%s_%s.mp4", asset.ContentHash, version))
	if err != nil {
		return err
	}
	return writeDerived(files, privatePath, reader)
}

func saveAudioVersion(files *storage.RepositoryFS, reader io.Reader, asset *repo.Asset, version string) error {
	privatePath, err := derivedPath("audios", version, fmt.Sprintf("%s_%s.mp3", asset.ContentHash, version))
	if err != nil {
		return err
	}
	return writeDerived(files, privatePath, reader)
}

func derivedPath(kind, version, filename string) (storage.RepositoryPath, error) {
	return storage.ParsePrivateRepositoryPath(path.Join(".lumilio/assets", kind, version, filename))
}

func writeDerived(files *storage.RepositoryFS, privatePath storage.RepositoryPath, reader io.Reader) error {
	if files == nil || reader == nil {
		return fmt.Errorf("derived file destination and reader are required")
	}
	directory, err := storage.ParsePrivateRepositoryPath(path.Dir(privatePath.String()))
	if err != nil {
		return err
	}
	if err := files.MkdirAllPrivate(directory, 0o755); err != nil {
		return fmt.Errorf("create derived directory: %w", err)
	}
	if _, err := files.WritePrivateFileAtomic(privatePath, reader, 0o644); err != nil {
		return fmt.Errorf("write derived file: %w", err)
	}
	return nil
}
