-- Copyright 2025 Kristopher Rahim Afful-Brown. All rights reserved.
--
-- Use of this source code is governed by a BSD-style
-- license that can be found in the LICENSE file.

create schema iot;

create table iot.device (
  id uuid
  , tag varchar(256) 
    not null 
    check (name <> '') -- unique?
  , long float8 
    default 0
  , lat float8
    default 0
  , primary key (id)
);