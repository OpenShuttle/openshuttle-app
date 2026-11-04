#!/bin/sh

# Forget the application and its .openshuttle file type.
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications
fi
if command -v update-mime-database >/dev/null 2>&1; then
  update-mime-database -n /usr/share/mime
fi

exit 0
