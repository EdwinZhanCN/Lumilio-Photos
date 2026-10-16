// Command docker-entrypoint prepares Docker bind mounts for the app user,
// drops privileges, and then executes the requested Server command. It is the
// container image's ENTRYPOINT and holds no product logic.
//
//atlas:group runtime
package main
