package view

import (
	"github.com/acoshift/acourse/pkg/model"
	"github.com/acoshift/flash"
)

// Page type provides layout data like title, description, and og
type Page struct {
	Title string
	Desc  string
	Image string
	URL   string
}

// IndexData type
type IndexData struct {
	*Page
	Courses []*model.Course
}

// AuthData type
type AuthData struct {
	*Page
	flash.Flash
}

// ProfileData type
type ProfileData struct {
	*Page
	flash.Flash
	OwnCourses      []*model.Course
	EnrolledCourses []*model.Course
}

// ProfileEditData type
type ProfileEditData struct {
	*Page
	flash.Flash
}

// CourseData type
type CourseData struct {
	*Page
	Course   *model.Course
	Enrolled bool
	Owned    bool
}

// CourseCreateData type
type CourseCreateData struct {
	*Page
	flash.Flash
}

// CourseEditData type
type CourseEditData struct {
	*Page
	flash.Flash
	Course *model.Course
}

// AdminUsersData type
type AdminUsersData struct {
	*Page
	Users       []*model.User
	CurrentPage int
	TotalPage   int
}

// AdminCoursesData type
type AdminCoursesData struct {
	*Page
	Courses []*model.Course
}

// AdminPaymentsData type
type AdminPaymentsData struct {
	*Page
	Payments    []*model.Payment
	CurrentPage int
	TotalPage   int
}
