// Package imaging is the libvips boundary. [StartVips] initialises the
// process-wide runtime; [ProcessImageStream] and [ProcessImageBytes] resize
// and encode derivatives; [StreamThumbnails] produces the thumbnail ladder;
// [ExportImageBytes] encodes user exports; [DecodeRGBResizeExact] and
// [DecodeRGBShortestEdgeCenterCrop] decode model inputs as [RGBImage].
//
// Callers pass bytes or readers, never repository paths, so the package stays
// free of storage and catalog knowledge.
//
//atlas:group media
package imaging
