package processors

import (
	"context"
	"fmt"
	"io"

	"server/internal/artifact"
	"server/internal/db/repo"
	"server/internal/storage"
)

// saveThumbnail, saveVideoVersion, and saveAudioVersion all share the one
// immutable artifact implementation. There is no second path writer for media
// derivatives: every result is addressed by source fence, stage, and pipeline
// version before the coordinator activates its catalog reference.
func (ap *AssetProcessor) saveThumbnail(ctx context.Context, files *storage.RepositoryFS, reader io.Reader, asset *repo.Asset, pipelineVersion, size string) (DerivedArtifact, error) {
	published, err := publishDerived(ctx, files, reader, asset, "derivatives", pipelineVersion, size+".webp")
	if err != nil {
		return DerivedArtifact{}, err
	}
	return DerivedArtifact{AssetID: asset.AssetID, SourceFence: asset.ContentID, RepositoryID: files.RepositoryID(), Size: size, StoragePath: published.Path, MimeType: "image/webp"}, nil
}

func saveVideoVersion(ctx context.Context, files *storage.RepositoryFS, reader io.Reader, asset *repo.Asset, pipelineVersion, version string) error {
	_, err := publishDerived(ctx, files, reader, asset, "transcode", pipelineVersion, version+".mp4")
	return err
}

func saveAudioVersion(ctx context.Context, files *storage.RepositoryFS, reader io.Reader, asset *repo.Asset, pipelineVersion, version string) error {
	_, err := publishDerived(ctx, files, reader, asset, "transcode", pipelineVersion, version+".mp3")
	return err
}

func publishDerived(ctx context.Context, files *storage.RepositoryFS, reader io.Reader, asset *repo.Asset, stage, pipelineVersion, name string) (artifact.Published, error) {
	if files == nil || reader == nil || asset == nil || asset.ContentID.String() == "" || pipelineVersion == "" {
		return artifact.Published{}, fmt.Errorf("derived artifact destination, reader, asset, and pipeline version are required")
	}
	store, err := artifact.NewStore(files)
	if err != nil {
		return artifact.Published{}, err
	}
	return store.Publish(ctx, artifact.Identity{SourceFence: asset.ContentID.String(), Stage: stage, PipelineVersion: pipelineVersion, Name: name}, reader)
}
