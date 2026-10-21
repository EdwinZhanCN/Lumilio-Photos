// Package rootcfg owns the portable .lumilioroot marker used to identify an
// authorized repository container independently from its current mount path.
//
// [ReadMarker] preserves typed access/compatibility facts independently from
// child health. [RootConfig.SaveGuarded] supplies guarded atomic writes for
// explicit mutations; observation never creates or repairs markers.
//
//atlas:group storage
package rootcfg
