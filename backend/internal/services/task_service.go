package services

import (
	"time"

	"github.com/trisfproject/gatherhub/internal/models"
	"gorm.io/gorm"
)

// TaskService handles database operations for event tasks.
type TaskService struct {
	db *gorm.DB
}

// NewTaskService creates a new TaskService instance.
func NewTaskService(db *gorm.DB) *TaskService {
	return &TaskService{db: db}
}

// Create persists a new task in the database.
func (s *TaskService) Create(task *models.Task) error {
	return s.db.Create(task).Error
}

// Update updates an existing task in the database.
func (s *TaskService) Update(task *models.Task) error {
	return s.db.Save(task).Error
}

// Delete removes a task from the database.
func (s *TaskService) Delete(id uint) error {
	return s.db.Delete(&models.Task{}, id).Error
}

// GetByID retrieves a task by its ID.
func (s *TaskService) GetByID(id uint) (*models.Task, error) {
	var task models.Task
	if err := s.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTasksByEvent retrieves all tasks for a specific event, optionally filtered.
func (s *TaskService) GetTasksByEvent(eventID uint, category, priority, status string) ([]models.Task, error) {
	var tasks []models.Task
	query := s.db.Where("event_id = ?", eventID)

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("due_date ASC, id ASC").Find(&tasks).Error
	return tasks, err
}

// GetTaskStats calculates task counts for the dashboard.
func (s *TaskService) GetTaskStats(eventID uint) (openCount, overdueCount, completedCount int, err error) {
	var open, overdue, completed int64
	now := time.Now()

	err = s.db.Model(&models.Task{}).
		Where("event_id = ? AND status IN ('Todo', 'In Progress')", eventID).
		Count(&open).Error
	if err != nil {
		return 0, 0, 0, err
	}

	err = s.db.Model(&models.Task{}).
		Where("event_id = ? AND status = 'Done'", eventID).
		Count(&completed).Error
	if err != nil {
		return 0, 0, 0, err
	}

	err = s.db.Model(&models.Task{}).
		Where("event_id = ? AND status IN ('Todo', 'In Progress') AND due_date < ?", eventID, now).
		Count(&overdue).Error
	if err != nil {
		return 0, 0, 0, err
	}

	return int(open), int(overdue), int(completed), nil
}

// GetTaskStatsDetailed calculates detailed task counts.
func (s *TaskService) GetTaskStatsDetailed(eventID uint) (totalCount, completedCount, inProgressCount, overdueCount int, err error) {
	var total, completed, inProgress, overdue int64
	now := time.Now()

	err = s.db.Model(&models.Task{}).
		Where("event_id = ?", eventID).
		Count(&total).Error
	if err != nil {
		return 0, 0, 0, 0, err
	}

	err = s.db.Model(&models.Task{}).
		Where("event_id = ? AND status = 'Done'", eventID).
		Count(&completed).Error
	if err != nil {
		return 0, 0, 0, 0, err
	}

	err = s.db.Model(&models.Task{}).
		Where("event_id = ? AND status = 'In Progress'", eventID).
		Count(&inProgress).Error
	if err != nil {
		return 0, 0, 0, 0, err
	}

	err = s.db.Model(&models.Task{}).
		Where("event_id = ? AND status IN ('Todo', 'In Progress') AND due_date < ?", eventID, now).
		Count(&overdue).Error
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return int(total), int(completed), int(inProgress), int(overdue), nil
}
