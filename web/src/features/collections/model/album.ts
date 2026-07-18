export interface AlbumViewModel {
  id: string;
  name: string;
  description?: string;
  imageCount: number;
  coverImages?: string[];
  createdAt: Date;
  updatedAt: Date;
  albumType?: string;
}
