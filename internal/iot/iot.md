# iot

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
