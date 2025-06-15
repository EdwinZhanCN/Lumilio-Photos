# Photo Frames System

A simple and intuitive photo framing system for RKPhoto Manager that allows users to quickly apply beautiful frames to their photos.

## Features

- **Simple Frame Selection**: Click to apply, click again to remove
- **Pre-designed Frames**: Beautiful, fixed settings that look great on any photo
- **Real-time Preview**: See framed images instantly in the main editor area
- **One-click Export**: Download framed photos as high-quality PNG files
- **Performance Optimized**: Fast rendering designed for web browsers
- **Community Friendly**: Easy for contributors to add new frame styles

## Quick Start

### Using the Frame System

1. **Load an Image**: Open an image in the Studio
2. **Navigate to Frames**: Click the "Frames" tab in the sidebar
3. **Select a Frame**: Click on any frame to apply it instantly
4. **Preview**: Your framed image appears in the main editor area (smaller size to show the frame effect)
5. **Export**: Click the download button to save your framed photo
6. **Remove**: Click the same frame again or the X button to remove it

### Integration Example

```typescript
import { SimpleFramePicker } from '@/components/PhotoFrames';

function MyComponent() {
  const [framedImageUrl, setFramedImageUrl] = useState<string | null>(null);
  
  const handleExport = (dataUrl: string, filename: string) => {
    // Handle the exported frame
    const link = document.createElement('a');
    link.download = filename;
    link.href = dataUrl;
    link.click();
  };

  return (
    <SimpleFramePicker
      imageUrl="/path/to/image.jpg"
      metadata="Custom metadata text"
      onFramedImageChange={setFramedImageUrl}
      onExport={handleExport}
    />
  );
}
```

## Available Frames

### Glass Frame
- **Style**: Modern glassmorphism effect
- **Features**: Transparent borders with blur effects, rounded corners, elegant shadows
- **Best For**: Contemporary photos, social media, modern aesthetics

### Classic Frame
- **Style**: Traditional wooden picture frame
- **Features**: Layered borders, wood grain texture, matte inner border
- **Best For**: Portraits, formal photography, professional presentations

### Polaroid Frame
- **Style**: Instant camera photo style
- **Features**: White borders, bottom text area, vintage aging effects, square crop
- **Best For**: Casual photos, memories, retro styling, nostalgic collections

## Architecture

### Simplified Design

The system uses a streamlined approach perfect for web applications:

- **No Complex Controls**: No sliders, color pickers, or adjustment panels
- **Fixed Beautiful Settings**: Each frame has carefully chosen defaults that look great
- **Direct Canvas Rendering**: Efficient drawing functions built into the picker component
- **Blob URL Generation**: Framed images are created as temporary URLs for display

### File Structure

```
PhotoFrames/
├── types.ts                 # TypeScript interfaces
├── frameRegistry.ts         # Frame definitions and registry
├── SimpleFramePicker.tsx    # Main component with canvas rendering
├── index.ts                 # Public exports
├── CONTRIBUTING.md          # Guide for adding new frames
└── README.md               # This documentation
```

### Frame Registry

Frames are defined with simple metadata:

```typescript
{
  id: "glass-frame",
  name: "Glass Frame", 
  description: "A modern glassmorphism-style frame...",
  author: "RKPhoto Team",
  version: "1.0.0",
  tags: ["modern", "glass", "elegant"],
  supportsMetadata: true,
  component: null // Not needed in simplified approach
}
```

## Canvas Rendering

Each frame has a dedicated drawing function that creates beautiful, consistent results:

```typescript
const drawGlassFrame = async (ctx: CanvasRenderingContext2D, img: HTMLImageElement) => {
  // Fixed, attractive settings
  const borderWidth = 40;
  const cornerRadius = 20;
  
  // Set canvas size
  ctx.canvas.width = img.width + borderWidth * 2;
  ctx.canvas.height = img.height + borderWidth * 2;
  
  // Draw glass effect with gradients and shadows
  // ... drawing logic
};
```

## User Experience

### Design Philosophy

1. **Simplicity First**: One-click frame application with no learning curve
2. **Instant Feedback**: Real-time preview in the main editor area  
3. **Web-Appropriate**: Fast performance suitable for browser environments
4. **Beautiful Defaults**: Pre-tuned settings that enhance photos

### Visual Flow

```
Original Image (full size) → Select Frame → Framed Image (70% size) → Export
```

The framed image displays smaller so users can clearly see the frame effect and decide if they like it.

## Performance

### Optimizations

- **Efficient Canvas Operations**: Minimal drawing calls for smooth performance
- **Blob URL Management**: Proper memory cleanup prevents leaks
- **Async Image Loading**: Non-blocking image processing
- **Fixed Calculations**: No real-time adjustments means consistent performance

### Browser Support

- Modern browsers with Canvas API support
- ES2017+ JavaScript features
- TypeScript compilation to ES2017 target

## Contributing

We welcome community contributions! The system is designed to make adding new frames straightforward.

### Adding a New Frame (Quick Overview)

1. **Add Definition**: Register your frame in `frameRegistry.ts`
2. **Create Drawing Function**: Add your canvas rendering logic to `SimpleFramePicker.tsx`
3. **Update Switch**: Add your case to the frame selection logic
4. **Test**: Verify it works with various image types

See [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed instructions.

### Frame Ideas We'd Love to See

- **Film Strip**: Multiple exposure effects with sprocket holes
- **Gallery**: Museum-style matting and frames
- **Neon**: Digital/cyberpunk aesthetic with glowing borders
- **Watercolor**: Artistic paint effects around edges
- **Photo Booth**: Multiple photos in strip format
- **Instagram**: Square crop with trendy border styles

## API Reference

### SimpleFramePicker Component

```typescript
interface SimpleFramePickerProps {
  imageUrl: string | null;              // Source image URL
  metadata?: string;                    // Optional metadata text
  onFramedImageChange?: (url: string | null) => void;  // Callback when frame applied/removed
  onExport?: (dataUrl: string, filename: string) => void;  // Export handler
}
```

### Registry Functions

```typescript
// Get all available frames
getAllFrames(): FrameDefinition[]

// Get specific frame by ID
getFrame(id: string): FrameDefinition | undefined

// Search frames by text
searchFrames(query: string): FrameDefinition[]

// Get frames by tag
getFramesByTag(tag: string): FrameDefinition[]

// Get categorized frames
getFrameCategories(): { [category: string]: FrameDefinition[] }
```

## Technical Details

### Dependencies

- React 18+
- TypeScript 4.5+
- HTML5 Canvas API
- Tailwind CSS (for UI styling)
- Heroicons (for interface icons)

### Canvas Considerations

- Images load with `crossOrigin: 'anonymous'` for CORS compatibility
- Canvas dimensions calculated based on image size and frame requirements
- Blob URLs created for temporary display and download functionality
- Memory management handles cleanup of temporary URLs

## Troubleshooting

### Common Issues

**Frame not applying**: Check browser console for CORS errors with images
**Export not working**: Ensure canvas toBlob() is supported
**Performance slow**: Verify image sizes are reasonable for web display
**Memory issues**: Check that blob URLs are being properly cleaned up

### Debug Tips

- Use browser dev tools to inspect canvas operations
- Check network tab for image loading issues
- Monitor memory usage for blob URL leaks
- Test with various image formats (JPEG, PNG, WebP)

## License

This photo frames system is part of RKPhoto Manager and follows the same license terms.

---

**Made with ❤️ by the RKPhoto Team**

Simple, beautiful, and fast - the way web photo frames should be.