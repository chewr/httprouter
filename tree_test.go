// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package httprouter

import (
	"fmt"
	"math/rand"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func printTree(panics interface{}, route string, tree *node) {
	var msg string
	if panics == nil {
		msg = "Added route"
	} else {
		msg = "Failed to add route"
	}
	fmt.Printf("%s %s \n%s\n", msg, route, PrettyPrint(tree))
}

func printChildren(n *node, prefix string) {
	fmt.Printf(" %02d:%02d %s%s[%d] %v %t %d \r\n", n.priority, n.maxParams, prefix, n.path, len(n.children), n.handle, n.wildChild, n.nType)
	for l := len(n.path); l > 0; l-- {
		prefix += " "
	}
	for _, child := range n.children {
		printChildren(child, prefix)
	}
}

// Used as a workaround since we can't compare functions or their addresses
var fakeHandlerValue string

func fakeHandler(val string) Handle {
	return func(http.ResponseWriter, *http.Request, Params) {
		fakeHandlerValue = val
	}
}

type testRequests []struct {
	path       string
	nilHandler bool
	route      string
	ps         Params
}

func checkRequests(t *testing.T, tree *node, requests testRequests) {
	for _, request := range requests {
		handler, ps, _ := tree.getValue(request.path)

		if handler == nil {
			if !request.nilHandler {
				t.Errorf("handle mismatch for route '%s': Expected non-nil handle", request.path)
			}
		} else if request.nilHandler {
			t.Errorf("handle mismatch for route '%s': Expected nil handle", request.path)
		} else {
			handler(nil, nil, nil)
			if fakeHandlerValue != request.route {
				t.Errorf("handle mismatch for route '%s': Wrong handle (%s != %s)", request.path, fakeHandlerValue, request.route)
			}
		}

		if !reflect.DeepEqual(ps, request.ps) {
			t.Errorf("Params mismatch for route '%s':\n%+v != %+v", request.path, ps, request.ps)
		}
	}
}

func checkPriorities(t *testing.T, n *node) uint32 {
	var prio uint32
	for i := range n.children {
		prio += checkPriorities(t, n.children[i])
	}

	if n.handle != nil {
		prio++
	}

	if n.priority != prio {
		t.Errorf(
			"priority mismatch for node '%s': is %d, should be %d",
			n.path, n.priority, prio,
		)
	}

	return prio
}

func checkMaxParams(t *testing.T, n *node) uint8 {
	var maxParams uint8
	for i := range n.children {
		params := checkMaxParams(t, n.children[i])
		if params > maxParams {
			maxParams = params
		}
	}
	if n.nType > root && !n.wildChild {
		maxParams++
	}

	if n.maxParams != maxParams {
		t.Errorf(
			"maxParams mismatch for node '%s': is %d, should be %d",
			n.path, n.maxParams, maxParams,
		)
	}

	return maxParams
}

func TestCountParams(t *testing.T) {
	if countParams("/path/:param1/static/*catch-all") != 2 {
		t.Fail()
	}
	if countParams(strings.Repeat("/:param", 256)) != 255 {
		t.Fail()
	}
}

func TestTreeAddAndGet(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/hi",
		"/contact",
		"/co",
		"/c",
		"/a",
		"/ab",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/α",
		"/β",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
		printTree(nil, route, tree)
	}

	//printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/a", false, "/a", nil},
		{"/", true, "", nil},
		{"/hi", false, "/hi", nil},
		{"/contact", false, "/contact", nil},
		{"/co", false, "/co", nil},
		{"/con", true, "", nil},  // key mismatch
		{"/cona", true, "", nil}, // key mismatch
		{"/no", true, "", nil},   // no matching child
		{"/ab", false, "/ab", nil},
		{"/α", false, "/α", nil},
		{"/β", false, "/β", nil},
	})

	checkPriorities(t, tree)
	checkMaxParams(t, tree)
}

func TestTreeWildcard(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/",
		"/cmd/:tool/:sub",
		"/cmd/:tool/",
		"/src/*filepath",
		"/search/",
		"/search/:query",
		"/user_:name",
		"/user_:name/about",
		"/files/:dir/*filepath",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/info/:user/public",
		"/info/:user/project/:project",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
		printTree(nil, route, tree)
	}

	//printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/cmd/test/", false, "/cmd/:tool/", Params{Param{"tool", "test"}}},
		{"/cmd/test", true, "", Params{Param{"tool", "test"}}},
		{"/cmd/test/3", false, "/cmd/:tool/:sub", Params{Param{"tool", "test"}, Param{"sub", "3"}}},
		{"/src/", false, "/src/*filepath", Params{Param{"filepath", "/"}}},
		{"/src/some/file.png", false, "/src/*filepath", Params{Param{"filepath", "/some/file.png"}}},
		{"/search/", false, "/search/", nil},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", Params{Param{"query", "someth!ng+in+ünìcodé"}}},
		{"/search/someth!ng+in+ünìcodé/", true, "", Params{Param{"query", "someth!ng+in+ünìcodé"}}},
		{"/user_gopher", false, "/user_:name", Params{Param{"name", "gopher"}}},
		{"/user_gopher/about", false, "/user_:name/about", Params{Param{"name", "gopher"}}},
		{"/files/js/inc/framework.js", false, "/files/:dir/*filepath", Params{Param{"dir", "js"}, Param{"filepath", "/inc/framework.js"}}},
		{"/info/gordon/public", false, "/info/:user/public", Params{Param{"user", "gordon"}}},
		{"/info/gordon/project/go", false, "/info/:user/project/:project", Params{Param{"user", "gordon"}, Param{"project", "go"}}},
	})

	checkPriorities(t, tree)
	checkMaxParams(t, tree)
}

func catchPanic(testFunc func()) (recv interface{}) {
	defer func() {
		recv = recover()
	}()

	testFunc()
	return
}

type testRoute struct {
	path     string
	conflict bool
}

func testRoutes(t *testing.T, routes []testRoute) {
	tree := &node{}

	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route.path, nil)
		})
		printTree(recv, route.path, tree)

		if route.conflict {
			if recv == nil {
				t.Errorf("no panic for conflicting route '%s'", route.path)
			}
		} else if recv != nil {
			t.Errorf("unexpected panic for route '%s': %v", route.path, recv)
			fmt.Printf("Unexpected panic for route %s", route.path)
		}
	}

	//printChildren(tree, "")
}

func TestTreeWildcardConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/:tool/:sub", false},
		{"/cmd/vet", false},
		{"/src/*filepath", false},
		{"/src/*filepathx", true},
		{"/src/", true},
		{"/src1/", false},
		{"/src1/*filepath", true},
		{"/src2*filepath", true},
		{"/search/:query", false},
		{"/search/valid", false},
		{"/user_:name", false},
		{"/user_x", false},
		{"/user_:name", false},
		{"/id:id", false},
		{"/id/:id", false},
		{"/tasks/:taskid", false},
		{"/tasks/find_all", false},
		{"/tasks/finished", false},
		{"/tasks/:bad", true},
	}
	testRoutes(t, routes)
}

func TestTreeChildConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/vet", false},
		{"/cmd/:tool/:sub", false},
		{"/src/AUTHORS", false},
		{"/src/*filepath", true},
		{"/user_x", false},
		{"/user_:name", false},
		{"/id/:id", false},
		{"/id:id", false},
		{"/products/deleted", false},
		{"/products/d_bloop", false},
		{"/products/:product/version", false},
		{"/products/:productid", true},
		{"/:id", false},
		{"/*filepath", true},
	}
	testRoutes(t, routes)
}

func TestTreeDuplicatePath(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/",
		"/doc/",
		"/src/*filepath",
		"/search/:query",
		"/user_:name",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		printTree(recv, route, tree)
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}

		// Add again
		recv = catchPanic(func() {
			tree.addRoute(route, nil)
		})
		printTree(recv, route, tree)
		if recv == nil {
			t.Fatalf("no panic while inserting duplicate route '%s", route)
		}
	}

	//printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/doc/", false, "/doc/", nil},
		{"/src/some/file.png", false, "/src/*filepath", Params{Param{"filepath", "/some/file.png"}}},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", Params{Param{"query", "someth!ng+in+ünìcodé"}}},
		{"/user_gopher", false, "/user_:name", Params{Param{"name", "gopher"}}},
	})
}

func TestEmptyWildcardName(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/user:",
		"/user:/",
		"/cmd/:/",
		"/src/*",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, nil)
		})
		printTree(recv, route, tree)
		if recv == nil {
			t.Fatalf("no panic while inserting route with empty wildcard name '%s", route)
		}
	}
}

func TestTreeCatchAllConflict(t *testing.T) {
	routes := []testRoute{
		{"/src/*filepath/x", true},
		{"/src2/", false},
		{"/src2/*filepath/x", true},
		{"/src3/*filepath", false},
		{"/src3/*filepath/x", true},
		{"/src/*filepath", false},
		{"/src4/static", false},
		{"/src4/*filepath", true},
		{"/src5/*filepath", true},
	}
	testRoutes(t, routes)
}

func TestTreeCatchAllConflictRoot(t *testing.T) {
	routes := []testRoute{
		{"/", false},
		{"/*filepath", true},
	}
	testRoutes(t, routes)
}

func TestTreeDoubleWildcard(t *testing.T) {
	const panicMsg = "only one wildcard per path segment is allowed"

	routes := [...]string{
		"/:foo:bar",
		"/:foo:bar/",
		"/:foo*bar",
	}

	for _, route := range routes {
		tree := &node{}
		recv := catchPanic(func() {
			tree.addRoute(route, nil)
		})
		printTree(recv, route, tree)

		if rs, ok := recv.(string); !ok || !strings.HasPrefix(rs, panicMsg) {
			t.Fatalf(`"Expected panic "%s" for route '%s', got "%v"`, panicMsg, route, recv)
		}
	}
}

/*func TestTreeDuplicateWildcard(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/:id/:name/:id",
	}
	for _, route := range routes {
		...
	}
}*/

func TestTreeTrailingSlashRedirect(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/hi",
		"/b/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/admin",
		"/admin/:category",
		"/admin/:category/:page",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/no/a",
		"/no/b",
		"/api/hello/:name",
		"/api/hello/world",
		"/api/hello/foo/",
		"/api/hello/bar/",
		"/api/hello/barre/",
		"/files/:param",
		"/files/foobar/*filepath",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		printTree(recv, route, tree)
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}

	//printChildren(tree, "")

	tsrRoutes := [...]string{
		"/hi/",
		"/b",
		"/search/gopher/",
		"/cmd/vet",
		"/src",
		"/x/",
		"/y",
		"/0/go/",
		"/1/go",
		"/a",
		"/admin/",
		"/admin/config/",
		"/admin/config/permissions/",
		"/doc/",
		"/api/hello/world/",
		"/api/hello/foo",
		"/api/hello/bar",
		"/files/foobar",
	}
	for _, route := range tsrRoutes {
		handler, _, tsr := tree.getValue(route)
		if handler != nil {
			t.Fatalf("non-nil handler for TSR route '%s", route)
		} else if !tsr {
			t.Errorf("expected TSR recommendation for route '%s'", route)
		}
	}

	noTsrRoutes := [...]string{
		"/",
		"/no",
		"/no/",
		"/_",
		"/_/",
		"/api/world/abc",
	}
	for _, route := range noTsrRoutes {
		handler, _, tsr := tree.getValue(route)
		if handler != nil {
			t.Fatalf("non-nil handler for No-TSR route '%s", route)
		} else if tsr {
			t.Errorf("expected no TSR recommendation for route '%s'", route)
		}
	}
}

func TestTreeRootTrailingSlashRedirect(t *testing.T) {
	tree := &node{}

	recv := catchPanic(func() {
		tree.addRoute("/:test", fakeHandler("/:test"))
	})
	printTree(recv, ":/test", tree)
	if recv != nil {
		t.Fatalf("panic inserting test route: %v", recv)
	}

	handler, _, tsr := tree.getValue("/")
	if handler != nil {
		t.Fatalf("non-nil handler")
	} else if tsr {
		t.Errorf("expected no TSR recommendation")
	}
}

func TestTreeFindCaseInsensitivePath(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/hi",
		"/b/",
		"/ABC/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/doc/go/away",
		"/no/a",
		"/no/b",
		"/Π",
		"/u/apfêl/",
		"/u/äpfêl/",
		"/u/öpfêl",
		"/v/Äpfêl/",
		"/v/Öpfêl",
		"/w/♬",  // 3 byte
		"/w/♭/", // 3 byte, last byte differs
		"/w/𠜎",  // 4 byte
		"/w/𠜏/", // 4 byte
	}

	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		printTree(recv, route, tree)
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}

	// Check out == in for all registered routes
	// With fixTrailingSlash = true
	for _, route := range routes {
		out, found := tree.findCaseInsensitivePath(route, true)
		if !found {
			t.Errorf("Route '%s' not found!", route)
		} else if string(out) != route {
			t.Errorf("Wrong result for route '%s': %s", route, string(out))
		}
	}
	// With fixTrailingSlash = false
	for _, route := range routes {
		out, found := tree.findCaseInsensitivePath(route, false)
		if !found {
			t.Errorf("Route '%s' not found!", route)
		} else if string(out) != route {
			t.Errorf("Wrong result for route '%s': %s", route, string(out))
		}
	}

	tests := []struct {
		in    string
		out   string
		found bool
		slash bool
	}{
		{"/HI", "/hi", true, false},
		{"/HI/", "/hi", true, true},
		{"/B", "/b/", true, true},
		{"/B/", "/b/", true, false},
		{"/abc", "/ABC/", true, true},
		{"/abc/", "/ABC/", true, false},
		{"/aBc", "/ABC/", true, true},
		{"/aBc/", "/ABC/", true, false},
		{"/abC", "/ABC/", true, true},
		{"/abC/", "/ABC/", true, false},
		{"/SEARCH/QUERY", "/search/QUERY", true, false},
		{"/SEARCH/QUERY/", "/search/QUERY", true, true},
		{"/CMD/TOOL/", "/cmd/TOOL/", true, false},
		{"/CMD/TOOL", "/cmd/TOOL/", true, true},
		{"/SRC/FILE/PATH", "/src/FILE/PATH", true, false},
		{"/SRC", "/src/", true, true},
		{"/x/Y", "/x/y", true, false},
		{"/x/Y/", "/x/y", true, true},
		{"/X/y", "/x/y", true, false},
		{"/X/y/", "/x/y", true, true},
		{"/X/Y", "/x/y", true, false},
		{"/X/Y/", "/x/y", true, true},
		{"/Y/", "/y/", true, false},
		{"/Y", "/y/", true, true},
		{"/Y/z", "/y/z", true, false},
		{"/Y/z/", "/y/z", true, true},
		{"/Y/Z", "/y/z", true, false},
		{"/Y/Z/", "/y/z", true, true},
		{"/y/Z", "/y/z", true, false},
		{"/y/Z/", "/y/z", true, true},
		{"/Aa", "/aa", true, false},
		{"/Aa/", "/aa", true, true},
		{"/AA", "/aa", true, false},
		{"/AA/", "/aa", true, true},
		{"/aA", "/aa", true, false},
		{"/aA/", "/aa", true, true},
		{"/A/", "/a/", true, false},
		{"/A", "/a/", true, true},
		{"/DOC", "/doc", true, false},
		{"/DOC/", "/doc", true, true},
		{"/NO", "", false, true},
		{"/DOC/GO", "", false, true},
		{"/π", "/Π", true, false},
		{"/π/", "/Π", true, true},
		{"/u/ÄPFÊL/", "/u/äpfêl/", true, false},
		{"/u/ÄPFÊL", "/u/äpfêl/", true, true},
		{"/u/ÖPFÊL/", "/u/öpfêl", true, true},
		{"/u/ÖPFÊL", "/u/öpfêl", true, false},
		{"/v/äpfêL/", "/v/Äpfêl/", true, false},
		{"/v/äpfêL", "/v/Äpfêl/", true, true},
		{"/v/öpfêL/", "/v/Öpfêl", true, true},
		{"/v/öpfêL", "/v/Öpfêl", true, false},
		{"/w/♬/", "/w/♬", true, true},
		{"/w/♭", "/w/♭/", true, true},
		{"/w/𠜎/", "/w/𠜎", true, true},
		{"/w/𠜏", "/w/𠜏/", true, true},
	}
	// With fixTrailingSlash = true
	for _, test := range tests {
		out, found := tree.findCaseInsensitivePath(test.in, true)
		if found != test.found || (found && (string(out) != test.out)) {
			t.Errorf("Wrong result for '%s': got %s, %t; want %s, %t",
				test.in, string(out), found, test.out, test.found)
			return
		}
	}
	// With fixTrailingSlash = false
	for _, test := range tests {
		out, found := tree.findCaseInsensitivePath(test.in, false)
		if test.slash {
			if found { // test needs a trailingSlash fix. It must not be found!
				t.Errorf("Found without fixTrailingSlash: %s; got %s", test.in, string(out))
			}
		} else {
			if found != test.found || (found && (string(out) != test.out)) {
				t.Errorf("Wrong result for '%s': got %s, %t; want %s, %t",
					test.in, string(out), found, test.out, test.found)
				return
			}
		}
	}
}

func TestTreeInvalidNodeType(t *testing.T) {
	const panicMsg = "invalid node type"

	tree := &node{}
	tree.addRoute("/", fakeHandler("/"))
	printTree(nil, "/", tree)
	tree.addRoute("/:page", fakeHandler("/:page"))
	printTree(nil, "/:page", tree)

	// set invalid node type
	tree.children[0].nType = 42

	// normal lookup
	recv := catchPanic(func() {
		tree.getValue("/test")
	})
	if rs, ok := recv.(string); !ok || rs != panicMsg {
		t.Fatalf("Expected panic '"+panicMsg+"', got '%v'", recv)
	}

	// case-insensitive lookup
	recv = catchPanic(func() {
		tree.findCaseInsensitivePath("/test", true)
	})
	if rs, ok := recv.(string); !ok || rs != panicMsg {
		t.Fatalf("Expected panic '"+panicMsg+"', got '%v'", recv)
	}
}

func TestTreeWildcardConflictEx(t *testing.T) {
	conflicts := [...]struct {
		route        string
		segPath      string
		existPath    string
		existSegPath string
	}{
		{"/con:xxx", ":xxx", `/con:tact`, `:tact`},
		{"/con:ooo/xxx", ":ooo", `/con:tact`, `:tact`},
		{"/con:tact_bad", ":tact_bad", `/con:tact`, `:tact`},
	}

	for _, conflict := range conflicts {
		// I have to re-create a 'tree', because the 'tree' will be
		// in an inconsistent state when the loop recovers from the
		// panic which threw by 'addRoute' function.
		tree := &node{}
		routes := [...]string{
			"/con:tact",
			"/who/are/*you",
			"/who/foo/hello",
		}

		for _, route := range routes {
			tree.addRoute(route, fakeHandler(route))
			printTree(nil, route, tree)
		}

		recv := catchPanic(func() {
			tree.addRoute(conflict.route, fakeHandler(conflict.route))
		})
		printTree(recv, conflict.route, tree)

		if !regexp.MustCompile(fmt.Sprintf("'%s' in new path .* conflicts with existing wildcard '%s' in existing prefix '%s'", conflict.segPath, conflict.existSegPath, conflict.existPath)).MatchString(fmt.Sprint(recv)) {
			t.Fatalf("invalid wildcard conflict error (%v)", recv)
		}
	}
}

func TestTreeCatchAllConflictEx(t *testing.T) {
	conflicts := [...]struct {
		route        string
		segPath      string
		existPath    string
		existSegPath string
	}{
		{"/who/are/foo", "/foo", `/who/are/\*you`, `/\*you`},
		{"/who/are/foo/", "/foo/", `/who/are/\*you`, `/\*you`},
		{"/who/are/foo/bar", "/foo/bar", `/who/are/\*you`, `/\*you`},
		{"/who/are/:foo", "/:foo", `/who/are/\*you`, `/\*you`},
		{"/who/are/*you/bar", `/\*you/bar`, `/who/are/\*you`, `/\*you`},
	}

	for _, conflict := range conflicts {
		// I have to re-create a 'tree', because the 'tree' will be
		// in an inconsistent state when the loop recovers from the
		// panic which threw by 'addRoute' function.
		tree := &node{}
		routes := [...]string{
			"/con:tact",
			"/who/are/*you",
			"/who/foo/hello",
		}

		for _, route := range routes {
			tree.addRoute(route, fakeHandler(route))
			printTree(nil, route, tree)
		}

		recv := catchPanic(func() {
			tree.addRoute(conflict.route, fakeHandler(conflict.route))
		})
		printTree(recv, conflict.route, tree)

		if !regexp.MustCompile(fmt.Sprintf("'%s' in new path .* cannot extend existing catch-all '%s' in existing prefix '%s'", conflict.segPath, conflict.existSegPath, conflict.existPath)).MatchString(fmt.Sprint(recv)) {
			t.Fatalf("invalid wildcard conflict error (%v)", recv)
		}
	}
}

var letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func genParam() string {
	const defaultLength int = 16
	b := make([]byte, defaultLength)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func genFilepath() string {
	return fmt.Sprintf("%s/%s/%s", genParam(), genParam(), genParam())
}

func createRequestAndParams(route string) (string, Params) {
	// scan for path vars
	pathBuf := make([]byte, 0)
	var ps []Param
	inParam, inCatchAll := false, false
	offset, i := 0, 0
	for i < len(route) {
		switch route[i] {
		case ':':
			offset = i + 1
			inParam = true
		case '*':
			offset = i + 1
			inCatchAll = true
		case '/':
			if inParam {
				requestVar := genParam()
				ps = append(ps, Param{route[offset:i], requestVar})
				pathBuf = append(pathBuf, requestVar...)
			}
			inParam = false
		default:
		}
		if !inParam && !inCatchAll {
			pathBuf = append(pathBuf, route[i])
		}
		i++
	}

	if inCatchAll {
		fpVar := genFilepath()
		ps = append(ps, Param{route[offset:i], "/" + fpVar})
		pathBuf = append(pathBuf, fpVar...)
	}

	if inParam {
		requestVar := genParam()
		ps = append(ps, Param{route[offset:i], requestVar})
		pathBuf = append(pathBuf, requestVar...)
	}

	return string(pathBuf), Params(ps)
}

func TestTreeAddAndGetWildcard(t *testing.T) {
	rand.Seed(1)
	routes := []string{
		"/",
		"/services/:service_id/preview",
		"/services/delete",
		"/services/deleter",
		"/services/delta",
		"/products/list",
		"/products/:product_id",
		"/files/v1/:dir",
		"/files/v1/:dir/*filepath",
		"/files/v2/:dir/*filepath",
		"/files/v2/:dir",
		"/files/v3/foobar",
		"/files/v3/foo:param",
		"/files/v3/foo/*filepath",
		"/users/v1/user_:name",
		"/users/v1/user_x",
		"/users/v1/user_/blah",
		"/param/partial:param",
		"/param/partial_foobar",
		"/param/partial_foo/baz",
		"/param/partial_foo/roo",
		"/param/partial_foo_:param",
		"/param/partial_foo_other:param",
		"/param/tsr_and_wildcard/:wildcard/endpoint",
		"/param/tsr_and_wildcard/static/",
		"/param/tsr_and_wildcard/static_blah",
		"/param/tsr_and_wildcard/static_blork",
		"/param/tsr_and_wildcard/_static/",
	}

	tree := &node{}

	requests := testRequests{}

	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})

		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}

		printTree(recv, route, tree)

		requestPath, ps := createRequestAndParams(route)

		requests = append(requests,
			struct {
				path       string
				nilHandler bool
				route      string
				ps         Params
			}{
				requestPath,
				false,
				route,
				ps,
			},
		)
	}

	checkRequests(t, tree, requests)

	addlRequests := testRequests{
		{
			"/param/partial_fool_me_once",
			false,
			"/param/partial:param",
			Params{Param{"param", "_fool_me_once"}},
		},
		{
			"/products/liXX",
			false,
			"/products/:product_id",
			Params{Param{"product_id", "liXX"}},
		},
		{
			"/param/partial_foo_parametric_equations",
			false,
			"/param/partial_foo_:param",
			Params{Param{"param", "parametric_equations"}},
		},
		{
			"/param/partial_foo_otherparam",
			false,
			"/param/partial_foo_other:param",
			Params{Param{"param", "param"}},
		},
		{
			"/param/tsr_and_wildcard/static",
			true,
			"",
			nil,
		},
		{
			"/param/tsr_and_wildcard/static_bl",
			true,
			"",
			Params{Param{"wildcard", "static_bl"}},
		},
		{
			"/param/tsr_and_wildcard/_static",
			true,
			"",
			nil,
		},
	}

	checkRequests(t, tree, addlRequests)
}
