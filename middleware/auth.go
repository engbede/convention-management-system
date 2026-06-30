package middleware

import (
	"net/http"
	"strconv"
)

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {

	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {

		cookie, err := r.Cookie("admin_session")

		if err != nil {

			http.Redirect(
				w,
				r,
				"/login",
				http.StatusSeeOther,
			)

			return
		}

		_, err = strconv.Atoi(cookie.Value)

		if err != nil {

			http.Redirect(
				w,
				r,
				"/login",
				http.StatusSeeOther,
			)

			return
		}

		next(w, r)
	}
}
