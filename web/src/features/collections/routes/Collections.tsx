import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { ErrorBoundary } from "react-error-boundary";
import { Album } from "lucide-react";
import { Album as AlbumType } from "../components/ImgStackGrid";
import { ImgStackGrid } from "../components/ImgStackGrid";

function Collections() {
  const albums: AlbumType[] = [
    {
      id: "1",
      name: "Summer Vacation 2024",
      imageCount: 45,
      createdAt: new Date(),
      updatedAt: new Date(),
    },
    {
      id: "2",
      name: "Summer Vacation 2024",
      imageCount: 45,
      createdAt: new Date(),
      updatedAt: new Date(),
    },
    {
      id: "3",
      name: "Summer Vacation 2024",
      imageCount: 45,
      createdAt: new Date(),
      updatedAt: new Date(),
    },
    // ... more albums
  ];
  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack code={500} title="Something went wrong" {...props} />
      )}
    >
      <PageHeader
        title="Collections"
        icon={<Album className="w-6 h-6 text-primary" strokeWidth={1.5} />}
      />

      <ImgStackGrid
        albums={albums}
        onAlbumClick={(album) => console.log("Selected album:", album)}
        loading={false}
        emptyMessage="Create your first album to get started"
      />
    </ErrorBoundary>
  );
}

export default Collections;
