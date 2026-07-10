package handlers

import (
    "html/template"
    "log"
    "net/http"
)

func Render(
    w http.ResponseWriter,
    page string,
    data interface{},
) {

    tmpl, err := template.ParseFiles(
        "templates/layouts/admin_layout.html",
        "templates/"+page,
    )

    if err != nil {
        log.Println(err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    err = tmpl.ExecuteTemplate(
        w,
        page,
        data,
    )

    if err != nil {
        log.Println(err)
    }
}