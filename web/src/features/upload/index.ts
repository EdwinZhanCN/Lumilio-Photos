/**
 * Upload feature module exports.
 *
 * This module provides the core upload functionality for the application,
 * including the provider component and context hook for accessing upload
 * state and operations throughout the application.
 *
 * @example
 * ```ts
 * // In your app root or layout component
 * import { UploadProvider } from '@/features/upload';
 *
 * function App() {
 *   return (
 *     <UploadProvider>
 *       <YourAppComponents />
 *     </UploadProvider>
 *   );
 * }
 *
 * // In components that need upload functionality
 * import { useUploadContext } from '@/features/upload';
 *
 * function UploadComponent() {
 *   const { addFiles, uploadFiles, isProcessing } = useUploadContext();
 *   // Use upload functionality
 * }
 * ```
 */

export { UploadProvider } from "./state/UploadProvider";
export { default as NavbarUploadQueue } from "./flows/queue/NavbarUploadQueue";
export { default as SupportedFormatsModal } from "./flows/intake/SupportedFormatsModal";
export { default as UnifiedUploadSection } from "./flows/intake/UnifiedUploadSection";
export { useUploadContext } from "./state/useUploadContext";
