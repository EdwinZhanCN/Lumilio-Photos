package service

var indexingTaskRuntimeRequirements = map[AssetIndexingTask][]string{
	AssetIndexingTaskSemanticImage:   {"semantic_image_embed"},
	AssetIndexingTaskBioCLIP:         {"bioclip_classify"},
	AssetIndexingTaskOCR:             {"ocr"},
	AssetIndexingTaskFaceRecognition: {"face_recognition"},
}

// IsIndexingTaskRuntimeAvailable returns true if at least one healthy node
// supports all required tasks for the given indexing task.
func IsIndexingTaskRuntimeAvailable(svc LumenService, task AssetIndexingTask) bool {
	if svc == nil {
		return false
	}
	requiredTasks, ok := indexingTaskRuntimeRequirements[task]
	if !ok {
		return false
	}
	for _, rt := range requiredTasks {
		if !svc.IsTaskAvailable(rt) {
			return false
		}
	}
	return true
}

// FilterRuntimeAvailableIndexingTasks filters the list to only tasks that
// have at least one healthy node supporting all required runtime tasks.
func FilterRuntimeAvailableIndexingTasks(tasks []AssetIndexingTask, svc LumenService) []AssetIndexingTask {
	available := make([]AssetIndexingTask, 0, len(tasks))
	for _, task := range tasks {
		if IsIndexingTaskRuntimeAvailable(svc, task) {
			available = append(available, task)
		}
	}
	return available
}
