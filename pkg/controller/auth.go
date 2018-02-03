package controller

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"unicode/utf8"

	"github.com/acoshift/go-firebase-admin"
	"github.com/asaskevich/govalidator"

	"github.com/acoshift/acourse/pkg/app"
)

func generateRandomString(n int) string {
	b := make([]byte, n)
	io.ReadFull(rand.Reader, b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func generateSessionID() string {
	return generateRandomString(24)
}

func generateMagicLinkID() string {
	return generateRandomString(64)
}

func (c *ctrl) SignIn(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		c.postSignIn(w, r)
		return
	}
	c.view.SignIn(w, r)
}

func (c *ctrl) postSignIn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := app.GetSession(ctx)
	f := s.Flash()

	email := r.FormValue("Email")
	if len(email) == 0 {
		f.Add("Errors", "email required")
	}
	if f.Has("Errors") {
		f.Set("Email", email)
		back(w, r)
		return
	}

	ok, err := c.repo.CanAcquireMagicLink(ctx, email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		f.Add("Errors", "อีเมลของคุณได้ขอ Magic Link จากเราไปแล้ว กรุณาตรวจสอบอีเมล")
		back(w, r)
	}

	f.Set("CheckEmail", "1")

	user, err := c.repo.FindUserByEmail(ctx, email)
	// don't lets user know if email is wrong
	if err == app.ErrNotFound {
		http.Redirect(w, r, "/signin/check-email", http.StatusSeeOther)
		return
	}
	if err != nil {
		f.Add("Errors", err.Error())
		back(w, r)
		return
	}

	linkID := generateMagicLinkID()

	err = c.repo.StoreMagicLink(ctx, linkID, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	linkQuery := make(url.Values)
	linkQuery.Set("id", linkID)
	if x := r.FormValue("r"); len(x) > 0 {
		linkQuery.Set("r", parsePath(x))
	}

	message := fmt.Sprintf(`สวัสดีครับคุณ %s,


ตามที่ท่านได้ขอ Magic Link เพื่อเข้าสู่ระบบสำหรับ acourse.io นั้นท่านสามารถเข้าได้ผ่าน Link ข้างล่างนี้ ภายใน 1 ชม.

%s

ทีมงาน acourse.io
	`, user.Name, c.makeLink("/signin/link", linkQuery))

	go c.sendEmail(user.Email.String, "Magic Link Request", markdown(message))

	http.Redirect(w, r, "/signin/check-email", http.StatusSeeOther)
}

func (c *ctrl) CheckEmail(w http.ResponseWriter, r *http.Request) {
	f := app.GetSession(r.Context()).Flash()
	if !f.Has("CheckEmail") {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	c.view.CheckEmail(w, r)
}

func (c *ctrl) SignInLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	linkID := r.FormValue("id")
	if len(linkID) == 0 {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	s := app.GetSession(ctx)
	f := s.Flash()

	userID, err := c.repo.FindMagicLink(ctx, linkID)
	if err != nil {
		f.Add("Errors", "ไม่พบ Magic Link ของคุณ")
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	app.SetUserID(s, userID)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (c *ctrl) SignInPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		c.postSignInPassword(w, r)
		return
	}
	c.view.SignInPassword(w, r)
}

func (c *ctrl) postSignInPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := app.GetSession(ctx)
	f := s.Flash()

	email := r.FormValue("Email")
	if len(email) == 0 {
		f.Add("Errors", "email required")
	}
	pass := r.FormValue("Password")
	if len(pass) == 0 {
		f.Add("Errors", "password required")
	}
	if f.Has("Errors") {
		f.Set("Email", email)
		back(w, r)
		return
	}

	userID, err := c.auth.VerifyPassword(ctx, email, pass)
	if err != nil {
		f.Add("Errors", err.Error())
		back(w, r)
		return
	}

	s.Rotate()
	app.SetUserID(s, userID)

	// if user not found in our database, insert new user
	// this happend when database out of sync with firebase authentication
	{
		ok, err := c.repo.IsUserExists(ctx, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			err = c.repo.CreateUser(ctx, &app.User{ID: userID, Email: sql.NullString{String: email, Valid: len(email) > 0}})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	rURL := parsePath(r.FormValue("r"))
	http.Redirect(w, r, rURL, http.StatusSeeOther)
}

var allowProvider = map[string]bool{
	"google.com":   true,
	"facebook.com": true,
	"github.com":   true,
}

func (c *ctrl) OpenID(w http.ResponseWriter, r *http.Request) {
	p := r.FormValue("p")
	if !allowProvider[p] {
		http.Error(w, "provider not allowed", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	sessID := generateSessionID()
	redirectURL, err := c.auth.CreateAuthURI(ctx, p, c.baseURL+"/openid/callback", sessID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s := app.GetSession(ctx)
	app.SetOpenIDSessionID(s, sessID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (c *ctrl) OpenIDCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := app.GetSession(ctx)
	sessID := app.GetOpenIDSessionID(s)
	app.DelOpenIDSessionID(s)
	user, err := c.auth.VerifyAuthCallbackURI(ctx, c.baseURL+r.RequestURI, sessID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, tx, err := app.NewTransactionContext(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	db := app.GetTransaction(ctx)
	// check is user sign up
	var cnt int64
	err = db.QueryRowContext(ctx, `select 1 from users where id = $1`, user.UserID).Scan(&cnt)
	if err == sql.ErrNoRows {
		// user not found, insert new user
		imageURL := c.uploadProfileFromURLAsync(user.PhotoURL)
		_, err = db.ExecContext(ctx, `
			insert into users
				(id, name, username, email, image)
			values
				($1, $2, $3, $4, $5)
		`, user.UserID, user.DisplayName, user.UserID, sql.NullString{String: user.Email, Valid: len(user.Email) > 0}, imageURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tx.Commit()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.Rotate()
	app.SetUserID(s, user.UserID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *ctrl) SignUp(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		c.postSignUp(w, r)
		return
	}
	c.view.SignUp(w, r)
}

func (c *ctrl) postSignUp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := app.GetSession(ctx).Flash()

	email := r.FormValue("Email")
	if len(email) == 0 {
		f.Add("Errors", "email required")
	}

	email, err := govalidator.NormalizeEmail(email)
	if err != nil {
		f.Add("Errors", err.Error())
		return
	}
	pass := r.FormValue("Password")
	if len(pass) == 0 {
		f.Add("Errors", "password required")
	}
	if n := utf8.RuneCountInString(pass); n < 6 || n > 64 {
		f.Add("Errors", "password must have 6 to 64 characters")
	}
	if f.Has("Errors") {
		f.Set("Email", email)
		back(w, r)
		return
	}

	userID, err := c.auth.CreateUser(ctx, &firebase.User{
		Email:    email,
		Password: pass,
	})
	if err != nil {
		f.Add("Errors", err.Error())
		back(w, r)
		return
	}

	db := app.GetDatabase(ctx)
	_, err = db.ExecContext(ctx, `
		insert into users
			(id, username, name, email)
		values
			($1, $2, '', $3)
	`, userID, userID, email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s := app.GetSession(ctx)
	app.SetUserID(s, userID)

	rURL := parsePath(r.FormValue("r"))
	http.Redirect(w, r, rURL, http.StatusSeeOther)
}

func (c *ctrl) SignOut(w http.ResponseWriter, r *http.Request) {
	app.GetSession(r.Context()).Destroy()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *ctrl) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		defer back(w, r)
		ctx := r.Context()
		f := app.GetSession(ctx).Flash()
		f.Set("OK", "1")
		email := r.FormValue("email")
		user, err := c.auth.GetUserByEmail(ctx, email)
		if err != nil {
			// don't send any error back to user
			return
		}
		c.auth.SendPasswordResetEmail(ctx, user.Email)
		return
	}
	c.view.ResetPassword(w, r)
}