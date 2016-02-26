package guviews

import (
	"fmt"
	"html/template"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gopherjs/gopherjs/js"
	"github.com/influx6/gu/gudispatch"
	"github.com/influx6/gu/guevents"
	"github.com/influx6/gu/gujs"
	"github.com/influx6/gu/gutrees"
	"github.com/influx6/gu/gutrees/elems"
	"github.com/influx6/gu/gutrees/styles"
)

//==============================================================================

// Renderable provides a interface for a renderable type.
type Renderable interface {
	Render() gutrees.Markup
}

//==============================================================================

// Behaviour provides a state changers for haiku.
type Behaviour interface {
	Hide()
	Show()
}

//==============================================================================

// Views define a Haiku Component
type Views interface {
	Behaviour
	MarkupRenderer

	UUID() string
	UID() string

	Bind(Views)
	Sync(Views)
	Mount(*js.Object)
	Events() guevents.EventManagers
}

//==============================================================================

// Path defines a representation of a location path matching a specific sequence.
type Path struct {
	gudispatch.PathDirective
	Param map[string]string
	ID    string
}

//==============================================================================

// pathMl provides a mutex to control read and write to internal view cache store.
var pathMl sync.RWMutex

// pathMl2 provides a mutex to control write and read to internal cache maps.
var pathMl2 sync.RWMutex

// pathWatch registers a view with selected watch routes to reduce unnecessary
// multiwatching of same routes and helps manage state of views.
var pathWatch = make(map[Views]map[string]bool)

// AttachView allows setting a specific view to become active for a specific URL
// route pattern. This allows to control the active and inactive state and also
// the visibility of the view dependent on the current location URL path.
func AttachView(v Views, pattern string) {
	gudispatch.URIMatcher(pattern)

	new := addView(v)

	pathMl.RLock()
	vcache := pathWatch[v]
	pathMl.RUnlock()

	// If we are already watching for this specific route then skip this.
	pathMl.RLock()
	hasOk := vcache[pattern]
	pathMl.RUnlock()

	if hasOk {
		return
	}

	pathMl2.Lock()
	vcache[pattern] = true
	pathMl2.Unlock()

	if !new {
		return
	}

	gudispatch.Subscribe(func(p gudispatch.PathDirective) {
		pathMl2.RLock()
		defer pathMl2.RUnlock()

		var found bool
		var params map[string]string

		for key := range vcache {
			// Get the matcher for this key.
			watcher := gudispatch.URIMatcher(key)

			pm, ok := watcher.Validate(p.String())
			if !ok {
				continue
			}

			params = pm
			found = true
			break
		}

		if !found {
			v.Hide()
			return
		}

		pu := Path{
			PathDirective: p,
			ID:            v.UUID(),
			Param:         params,
		}

		gudispatch.Dispatch(&pu)
	})

	gudispatch.Follow(gudispatch.GetLocation())
}

// addView attaches view into the pathWatch match.
func addView(v Views) bool {
	// Get the view route cache.
	pathMl.RLock()
	_, ok := pathWatch[v]
	pathMl.RUnlock()

	// If no cache is found for this view then make one and store it.
	if !ok {
		vcache := make(map[string]bool)
		pathMl.Lock()
		pathWatch[v] = vcache
		pathMl.Unlock()
		return true
	}

	return false
}

//==============================================================================

//==============================================================================

// ViewStates defines the two possible behavioral state of a view's markup
type ViewStates interface {
	Render(gutrees.Markup)
}

// HideView provides a ViewStates for Views inactive state
type HideView struct{}

// Render marks the given markup as display:none
func (v HideView) Render(m gutrees.Markup) {
	// if we are allowed to query for styles then check and change display
	if mm, ok := m.(gutrees.MarkupProps); ok {
		if ds, err := gutrees.GetStyle(mm, "display"); err == nil {
			if !strings.Contains(ds.Value, "none") {
				ds.Value = "none"
			}
			return
		}

	}
	styles.Display("none").Apply(m)
}

// ShowView provides a ViewStates for Views active state
type ShowView struct{}

// Render marks the given markup with a display: block
func (v ShowView) Render(m gutrees.Markup) {
	//if we are allowed to query for styles then check and change display
	if mm, ok := m.(gutrees.MarkupProps); ok {
		if ds, err := gutrees.GetStyle(mm, "display"); err == nil {
			if strings.Contains(ds.Value, "none") {
				ds.Value = "block"
			}
			return
		}

		styles.Display("block").Apply(m)
	}
}

//==============================================================================

// ViewUpdate defines a view update notification which contains the name of the
// view to be notified for an update.
type ViewUpdate struct {
	ID string
}

// ViewState defines a notification struct of the state of the view wether it
// is active or not.
type ViewState struct {
	ID string
	On bool
}

//==============================================================================

// hider defines a global hide renderer.
var hider HideView

// shower defines a global display renderer.
var shower ShowView

// view defines a basic struture for building UI view.
type view struct {
	ready        int64
	switchActive int64
	uid          string
	uuid         string
	dom          *js.Object
	renders      []Renderable
	liveMarkup   gutrees.Markup
	encoder      gutrees.MarkupWriter
	events       guevents.EventManagers
	activeState  ViewStates
}

// View returns a View instance. The view is giving a customID string, which
// gets associated with this view, and provides a convenient means of dispatching
// events directly to it, if this is a empty string, a random one will be
// generated for it.
func View(customID string, view ...Renderable) Views {
	return CustomView(customID, gutrees.SimpleMarkupWriter, view...)
}

// CustomView returns a gu.Views implementing struct that provides the ability to
// render and update UI efficiently. This function allows greater control of
// the customId for which the views and it's dom will be identified with and
// the writer used to decode our dom structures into valid html.
func CustomView(cid string, writer gutrees.MarkupWriter, vw ...Renderable) Views {
	if cid == "" {
		cid = gutrees.RandString(8)
	}

	vm := &view{
		encoder:     writer,
		renders:     vw,
		uid:         cid,
		activeState: shower,
		events:      guevents.NewEventManager(),
		uuid:        gutrees.RandString(20),
		// uuid:    uuid.NewV4().String(),
	}

	// Subscribe for view update requests from the central dispatcher.
	gudispatch.Subscribe(func(v *ViewUpdate) {
		if v.ID != vm.UUID() && v.ID != vm.UID() {
			return
		}

		// If we are not domless then patch.
		if vm.dom == nil {
			return
		}

		replaceOnly := atomic.LoadInt64(&vm.ready) == 0

		html := vm.RenderHTML()
		fmt.Printf("NewHTML %s\n", html)
		gujs.Patch(gujs.CreateFragment(string(html)), vm.dom, replaceOnly)
	})

	gudispatch.Subscribe(func(p *Path) {
		if p.ID != vm.UUID() && p.ID != vm.UID() {
			return
		}

		vm.Show()
	})

	return vm
}

// UID returns the custom id associated with this view.
func (v *view) UID() string {
	return v.uuid
}

// UUID returns the identification number associated with this view.
func (v *view) UUID() string {
	return v.uuid
}

// BindView binds the given views together,were the view provided as argument
// will notify this view of change and to act according.
func (v *view) Bind(vs Views) {
	gudispatch.Subscribe(func(vm *ViewUpdate) {
		if vm.ID != vs.UUID() && vm.ID != vs.UID() {
			return
		}

		// Notify this view of change.
		gudispatch.Dispatch(&ViewUpdate{ID: v.UUID()})
	})
}

// Sync connects a view not only to the update cycles of this views but also
// to the state of this view, that is, if this view becomes hidden, then
// the synced view follows suits and as such.
func (v *view) Sync(vs Views) {
	v.Bind(vs)
	gudispatch.Subscribe(func(vm *ViewState) {
		if vm.ID != vs.UUID() && vm.ID != vs.UID() {
			return
		}

		if !vm.On {
			vs.Hide()
			return
		}

		vs.Show()
	})
}

// Mount is to be called in the browser to loadup this view with a dom
func (v *view) Mount(dom *js.Object) {
	v.dom = dom
	v.events.OffloadDOM()
	v.events.LoadDOM(dom)

	// Set the ready signal as on.
	atomic.StoreInt64(&v.ready, 1)

	// Notify for update to dom.
	gudispatch.Dispatch(&ViewUpdate{
		ID: v.UUID(),
	})
}

// Show activates the view to generate a visible markup
func (v *view) Show() {
	atomic.StoreInt64(&v.switchActive, 1)
	{
		v.activeState = shower
	}
	atomic.StoreInt64(&v.switchActive, 0)

	gudispatch.Dispatch(&ViewUpdate{ID: v.UUID()})

	gudispatch.Dispatch(&ViewState{
		ID: v.UUID(),
		On: true,
	})
}

// Hide deactivates the view
func (v *view) Hide() {
	atomic.StoreInt64(&v.switchActive, 1)
	{
		v.activeState = hider
	}
	atomic.StoreInt64(&v.switchActive, 0)

	gudispatch.Dispatch(&ViewUpdate{ID: v.UUID()})
	gudispatch.Dispatch(&ViewState{
		ID: v.UUID(),
		On: false,
	})
}

// Events returns the views events manager
func (v *view) Events() guevents.EventManagers {
	return v.events
}

//==============================================================================

// MarkupRenderer provides a interface for a types capable of rendering dom markup.
type MarkupRenderer interface {
	Renderable
	RenderHTML() template.HTML
}

// Render renders the generated markup for this view, if the renderers are more
// than one then all are rendered into a div(as we need this to maintain sanity
// during reconciliation and updates) of rendered dom.
func (v *view) Render() gutrees.Markup {
	if len(v.renders) == 0 {
		return elems.Div()
	}

	var dom gutrees.Markup

	// If we are more than 1 then run through and apply all to a div.
	if len(v.renders) > 1 {
		dom = elems.Div()

		for _, rv := range v.renders {
			rv.Render().Apply(dom)
		}

	} else {
		dom = v.renders[0].Render()
	}

	atomic.StoreInt64(&v.switchActive, 1)
	{
		v.activeState.Render(dom)
	}
	atomic.StoreInt64(&v.switchActive, 0)

	if v.liveMarkup != nil {
		dom.Reconcile(v.liveMarkup)
	}

	// swap the uid for the new dom
	// to ensure we keep the sync between backend and frontend in sync.
	if backdoor, ok := dom.(gutrees.SwappableIdentity); ok {
		backdoor.SwapUID(v.uid)
	}

	if eventdoor, ok := dom.(gutrees.Eventers); ok {
		eventdoor.UseEventManager(v.events)
	}

	v.events.LoadUpEvents()
	v.liveMarkup = dom

	return dom
}

// RenderHTML renders out the views markup as a string wrapped with template.HTML
func (v *view) RenderHTML() template.HTML {
	ma, _ := v.encoder.Write(v.Render())
	return template.HTML(ma)
}