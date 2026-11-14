-- name: DeleteAsset :exec
DELETE FROM assets WHERE id = ?;

-- name: CountAssets :one
SELECT count(*) FROM assets;
