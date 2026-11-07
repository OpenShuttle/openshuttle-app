//go:build linux

package keepawake

import (
	"errors"

	"github.com/godbus/dbus/v5"
)

// Inhibit flags of the desktop portal.
const (
	portalInhibitSuspend = 4
	portalInhibitIdle    = 8
)

// start asks the desktop portal (GNOME, KDE and others, Flatpak included) to
// inhibit suspend and idle (the screen blanking and locking); without a
// portal, the freedesktop screensaver service. The hold lives on a private
// session bus connection: it ends when stopped, or when the process exits.
func start(reason string) (func(), error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	stop, portalErr := portalInhibit(conn, reason)
	if portalErr == nil {
		return stop, nil
	}
	stop, saverErr := screenSaverInhibit(conn, reason)
	if saverErr == nil {
		return stop, nil
	}
	_ = conn.Close()
	return nil, errors.Join(portalErr, saverErr)
}

func portalInhibit(conn *dbus.Conn, reason string) (func(), error) {
	portal := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	var handle dbus.ObjectPath
	err := portal.Call("org.freedesktop.portal.Inhibit.Inhibit", 0,
		"", uint32(portalInhibitSuspend|portalInhibitIdle),
		map[string]dbus.Variant{"reason": dbus.MakeVariant(reason)},
	).Store(&handle)
	if err != nil {
		return nil, err
	}
	return func() {
		conn.Object("org.freedesktop.portal.Desktop", handle).Call("org.freedesktop.portal.Request.Close", 0)
		_ = conn.Close()
	}, nil
}

func screenSaverInhibit(conn *dbus.Conn, reason string) (func(), error) {
	saver := conn.Object("org.freedesktop.ScreenSaver", "/org/freedesktop/ScreenSaver")
	var cookie uint32
	if err := saver.Call("org.freedesktop.ScreenSaver.Inhibit", 0, "OpenShuttle", reason).Store(&cookie); err != nil {
		return nil, err
	}
	return func() {
		saver.Call("org.freedesktop.ScreenSaver.UnInhibit", 0, cookie)
		_ = conn.Close()
	}, nil
}
