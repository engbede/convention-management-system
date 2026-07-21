package middleware

import (
	"net/http"

	"convention-management-system/sessions"
)

func RequireLogin(
	next http.HandlerFunc,
) http.HandlerFunc {

	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {

		session, err := sessions.Store.Get(
			r,
			"youth-community",
		)

		if err != nil {

			http.Redirect(
				w,
				r,
				"/community/login",
				http.StatusSeeOther,
			)

			return
		}

		if session.IsNew {

			http.Redirect(
				w,
				r,
				"/community/login",
				http.StatusSeeOther,
			)

			return
		}

		_, ok := session.Values["user_id"].(int)

		if !ok {

			session.Options.MaxAge = -1
			session.Save(r, w)

			http.Redirect(
				w,
				r,
				"/community/login",
				http.StatusSeeOther,
			)

			return
		}

		next(w, r)
	}
}
