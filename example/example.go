//go:generate gomo UserRepository
package example

type User struct{}

type UserRepository interface {
	Init()
	FindUser(id string) (User, error)
	SaveUser(User) error
}

type Unwanted interface {
}
