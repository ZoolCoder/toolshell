package toolshell

import (
	"net/http"
	"net/url"
)

// Back answers a form post: it redirects to the form's return_to field, or to
// fallback, with the outcome as the flash the next page's Chrome shows (Msg
// from ?msg=, Err from ?err=). A return_to naming another site is ignored.
func Back(w http.ResponseWriter, r *http.Request, fallback, msg string, err error) {
	target := SafeNext(r.FormValue("return_to"))
	if target == "" {
		target = fallback
	}
	u, perr := url.Parse(target)
	if perr != nil {
		u = &url.URL{Path: "/"}
	}
	q := u.Query()
	q.Del("msg")
	q.Del("err")
	if err != nil {
		q.Set("err", err.Error())
	} else if msg != "" {
		q.Set("msg", msg)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}
