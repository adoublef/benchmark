-- Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
--
-- Use of this source code is governed by a BSD-style
-- license that can be found in the LICENSE file.

create schema iot;

create type state as enum ('Created');

create table iot.device (
  id uuid
  , tag varchar(40)
    unique 
    check (length(tag) >= 8)
  , long float8 
    default 0
  , lat float8
    default 0
  , state state
    default 'Created'
  , primary key (id)
);