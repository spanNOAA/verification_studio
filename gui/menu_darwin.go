//go:build darwin

package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

void setupNativeMacMenu() {
    @autoreleasepool {
        NSMenu *mainMenu = [[NSMenu alloc] init];

        // 1. App Menu (Quit Cmd+Q)
        NSMenuItem *appMenuItem = [[NSMenuItem alloc] init];
        [mainMenu addItem:appMenuItem];

        NSMenu *appMenu = [[NSMenu alloc] init];
        NSString *appName = [[NSProcessInfo processInfo] processName];
        NSString *quitTitle = [NSString stringWithFormat:@"Quit %@", appName];
        NSMenuItem *quitMenuItem = [[NSMenuItem alloc] initWithTitle:quitTitle
                                                              action:@selector(terminate:)
                                                       keyEquivalent:@"q"];
        [appMenu addItem:quitMenuItem];
        [appMenuItem setSubmenu:appMenu];

        // 2. Edit Menu (Cmd+C, Cmd+V, Cmd+X, Cmd+A, Cmd+Z)
        NSMenuItem *editMenuItem = [[NSMenuItem alloc] init];
        [mainMenu addItem:editMenuItem];
        NSMenu *editMenu = [[NSMenu alloc] initWithTitle:@"Edit"];
        [editMenu addItemWithTitle:@"Undo" action:@selector(undo:) keyEquivalent:@"z"];
        [editMenu addItemWithTitle:@"Redo" action:@selector(redo:) keyEquivalent:@"Z"];
        [editMenu addItem:[NSMenuItem separatorItem]];
        [editMenu addItemWithTitle:@"Cut" action:@selector(cut:) keyEquivalent:@"x"];
        [editMenu addItemWithTitle:@"Copy" action:@selector(copy:) keyEquivalent:@"c"];
        [editMenu addItemWithTitle:@"Paste" action:@selector(paste:) keyEquivalent:@"v"];
        [editMenu addItemWithTitle:@"Select All" action:@selector(selectAll:) keyEquivalent:@"a"];
        [editMenuItem setSubmenu:editMenu];

        // 3. Window Menu (Cmd+W)
        NSMenuItem *windowMenuItem = [[NSMenuItem alloc] init];
        [mainMenu addItem:windowMenuItem];
        NSMenu *windowMenu = [[NSMenu alloc] initWithTitle:@"Window"];
        [windowMenu addItemWithTitle:@"Close" action:@selector(performClose:) keyEquivalent:@"w"];
        [windowMenuItem setSubmenu:windowMenu];

        [NSApp setMainMenu:mainMenu];
    }
}
*/
import "C"

func initMacMenu() {
	C.setupNativeMacMenu()
}
