package service

import "testing"

func TestNormalizeRequestedIndexingTasks_DefaultExcludesBioCLIP(t *testing.T) {
	tasks := normalizeRequestedIndexingTasks(nil)

	if containsIndexingTask(tasks, AssetIndexingTaskBioCLIP) {
		t.Fatalf("default indexing tasks should not include BioCLIP: %#v", tasks)
	}
}

func TestNormalizeRequestedIndexingTasks_IgnoresBioCLIP(t *testing.T) {
	tasks := normalizeRequestedIndexingTasks([]AssetIndexingTask{
		AssetIndexingTaskBioCLIP,
		AssetIndexingTaskOCR,
	})

	if containsIndexingTask(tasks, AssetIndexingTaskBioCLIP) {
		t.Fatalf("requested indexing tasks should not include BioCLIP: %#v", tasks)
	}
	if len(tasks) != 1 || tasks[0] != AssetIndexingTaskOCR {
		t.Fatalf("expected only OCR task, got %#v", tasks)
	}
}

func containsIndexingTask(tasks []AssetIndexingTask, target AssetIndexingTask) bool {
	for _, task := range tasks {
		if task == target {
			return true
		}
	}
	return false
}
