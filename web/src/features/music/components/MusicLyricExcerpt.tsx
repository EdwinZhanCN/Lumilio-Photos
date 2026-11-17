import { $api } from "@/lib/http-commons/queryClient";

/** The highlight reads the same stored lyrics as the player; no online fetch. */
export default function MusicLyricExcerpt({ trackId }: { trackId?: string }) {
  const lyrics = $api.useQuery(
    "get",
    "/api/v1/music/tracks/{id}/lyrics",
    { params: { path: { id: trackId ?? "" } } },
    { enabled: Boolean(trackId), staleTime: 30_000 },
  );
  const lines = lyrics.data?.content
    ?.split(/\r?\n/)
    .map((line) => line.replace(/\[[^\]]*\]/g, "").trim())
    .filter(Boolean)
    .slice(0, 3);
  return lines?.length ? (
    <p className="music-lyric-excerpt">
      {lines.map((line, index) => (
        <span key={index}>
          {line}
          <br />
        </span>
      ))}
    </p>
  ) : null;
}
