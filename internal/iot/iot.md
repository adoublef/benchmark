# iot

<!-- https://www.cockroachlabs.com/docs/stable/enum -->

```go
// func (d *DB) editDevice(ctx context.Context, dev Device) error {
//  const query = `
//
// update iot.device set long = $2, lat = $3
// where id = $1
// `
//
//  rs, err := d.RWC.Exec(ctx, query, dev.ID, dev.Loc.Longitude, dev.Loc.Latitude)
//  if err != nil {
//   return err
//  } else if rs.RowsAffected() < 1 {
//   return ErrNotExist
//  }
//  return nil
// }
```

```go
//  func (d *DB) Within(ctx context.Context, loc Location, r float64) ([]Device, error) {
//   const query = `
//  select id, long, lat
//  from iot.device
//  where st_dwithin(st_makepoint(long, lat), st_makepoint($2, $3), $1);`
//
//   rr, err := d.RWC.Query(ctx, query, r, loc.Longitude, loc.Latitude)
//   if err != nil {
//    return nil, err
//   }
//   var dd []Device
//   for rr.Next() {
//    var d Device
//    err := rr.Scan(&d.ID, &d.Loc.Longitude, &d.Loc.Latitude)
//    if err != nil {
//     return nil, err
//    }
//    dd = append(dd, d)
//   }
//   if err := rr.Err(); err != nil {
//    return nil, err
//   }
//   return dd, nil
//  }
```

```go
// func (d *DB) Pin(ctx context.Context, loc Location) (ID, error) {
//  const query = `
//  insert into iot.device (id, long, lat)
//  values ($1, $2, $3)`
//  id := uuid.Must(uuid.NewV7())
//  _, err := d.RWC.Exec(ctx, query, id, loc.Longitude, loc.Latitude)
//  if err != nil {
//   return ID{}, err
//  }
//  return id, nil
// }
```

```go
// func (d *DB) Device(ctx context.Context, id ID) (Device, error) {
//  const query = `
//  select id, long, lat
//  from iot.device
//  where id = $1`
//
//  var dev Device
//  err := d.RWC.QueryRow(ctx, query, id).Scan(&dev.ID, &dev.Loc.Longitude, &dev.Loc.Latitude)
//  if err != nil {
//   return Device{}, err
//  }
//  return dev, nil
// }
```

```txt
created Initial status
starting Transitioning from stopped or suspended to started
started Running and network-accessible
stopping Transitioning from started to stopped
stopped Exited, either on its own or explicitly stopped
suspending Transitioning from started to suspended
suspended Suspended to disk; will attempt to resume on next start
replacing User-initiated configuration change (image, VM size, etc.) in progress
destroying User asked for the Machine to be completely removed
destroyed No longer exists
```

```txt
creating Initial state for a new volume.
created Volume is in a stable state with no on-going operations.
extending The volume is being extended.
restoring The volume is being restored from a snapshot.
enabling_remote_export The volume fork process is initializing.
hydrating The volume is being forked (copied) and is actively hydrating. The volume can be mounted.
recovering The volume is being recovered from the pending_destroy state.
scheduling_destroy The volume is being deleted.
pending_destroy The volume is soft deleted.
```
