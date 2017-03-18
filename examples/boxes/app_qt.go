// Package main defines the main method which creates the qt window and loads the app.

//go:generate qtdeploy build desktop ./

// Code is generated automatically by the gu library. Change with understanding ;).

package main

import (
	"github.com/gu-io/gu"
	"github.com/gu-io/gu/drivers/qtwebview"
)

// manifestURL defines the URL where the manifest
// file will be located
var manifestURL = "assets/manifests.json"

func registerViews(app *gu.NApp){
	
	app.View(gu.ViewAttr{
		Name:  "View.Index",
        Route: "/*",
    })

}

func main() {

	// Initialize QT window processes.
	qtwebview.InitQTApplication()

	driver := qtwebview.NewWebviewDriver(qtwebview.QTAttr{
		URL: "",
		MinWidth: 800,
		MinHeight: 640,
		Manifest: manifestURL,
	})

	app := gu.App(gu.AppAttr{
		InterceptRequests: true,
		Name:              "boxes",
		Mode:              gu.DevelopmentMode,
		Title:             "boxes Gu App",
		Manifests: 			manifestURL,
		Driver: 			driver,
	})

	registerViews(app)

	driver.Run()
}