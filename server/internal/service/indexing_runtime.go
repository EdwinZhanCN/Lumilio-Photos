package service

// TaskAvailabilityChecker captures the minimal runtime capability check used
// before enqueueing heavyweight ML jobs.
type TaskAvailabilityChecker interface {
	IsTaskAvailable(taskName string) bool
}

var indexingTaskRuntimeRequirements = map[AssetIndexingTask][]string{
	AssetIndexingTaskClip:    {"clip_image_embed", "clip_classify", "clip_scene_classify"},
	AssetIndexingTaskOCR:     {"ocr"},
	AssetIndexingTaskCaption: {"vlm_generate"},
	AssetIndexingTaskFace:    {"face_detect_and_embed"},
}

func IsIndexingTaskRuntimeAvailable(checker TaskAvailabilityChecker, task AssetIndexingTask) bool {
	if checker == nil {
		return false
	}

	requiredTasks, ok := indexingTaskRuntimeRequirements[task]
	if !ok {
		return false
	}

	for _, runtimeTask := range requiredTasks {
		if !checker.IsTaskAvailable(runtimeTask) {
			return false
		}
	}

	return true
}

func FilterRuntimeAvailableIndexingTasks(tasks []AssetIndexingTask, checker TaskAvailabilityChecker) []AssetIndexingTask {
	available := make([]AssetIndexingTask, 0, len(tasks))
	for _, task := range tasks {
		if IsIndexingTaskRuntimeAvailable(checker, task) {
			available = append(available, task)
		}
	}
	return available
}
