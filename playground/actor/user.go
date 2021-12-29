// Code generated by Molizen. DO NOT EDIT.

// Package actor_user is a generated Molizen package.
package actor_user

import (
	sync "sync"

	actor "github.com/sanposhiho/molizen/actor"
	future "github.com/sanposhiho/molizen/future"
	user "github.com/sanposhiho/molizen/playground/user"
)

// UserActor is a actor of User interface.
type UserActor struct {
	lock     sync.Mutex
	internal user.User
}

func New(internal user.User) *UserActor {
	return &UserActor{
		internal: internal,
	}
}

// SetNameResult is the result type for SetName.
type SetNameResult struct {
	ret0 string
}

// SetName actor base method.
func (a *UserActor) SetName(ctx actor.Context, name string) future.Future[SetNameResult] {
	ctx.UnlockParent()
	ctx = actor.NewContext(a.lock.Lock, a.lock.Unlock)

	f := future.New[SetNameResult]()
	go func() {
		a.lock.Lock()
		defer a.lock.Unlock()

		ret0 := a.internal.SetName(ctx, name)

		ret := SetNameResult{
			ret0: ret0,
		}

		ctx.LockParent()

		f.Send(ret)
	}()

	return f
}
