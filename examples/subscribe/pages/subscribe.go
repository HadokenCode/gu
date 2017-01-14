package pages

import (
	"github.com/gu-io/gu"
	"github.com/gu-io/gu/examples/subscribe/app"
	"github.com/gu-io/gu/trees/elems"
	"github.com/gu-io/gu/trees/property"
)

var _ = gu.Resource(func() {

	gu.GlobalRoute("#")

	gu.View(elems.Div(
		elems.CSS(app.RootCSS, nil),
		property.ClassAttr("root"),
		elems.Header1(
			elems.Text("Become A Subscriber"),
		),
	), "", false, false)

	gu.View(&app.Subscriber{
		SubmitBtnColor: "",
	}, "", false, false)

})
