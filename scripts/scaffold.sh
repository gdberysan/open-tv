#!/bin/bash

# Este script crea la estructura de directorios canónica para el proyecto IPTV Ecosistema

mkdir -p pipeline/cmd/worker
mkdir -p pipeline/internal/validator
mkdir -p pipeline/internal/parser
mkdir -p pipeline/internal/provider

mkdir -p gateway/cmd/server
mkdir -p gateway/internal/domain
mkdir -p gateway/internal/ports
mkdir -p gateway/internal/adapters/db
mkdir -p gateway/internal/adapters/cache
mkdir -p gateway/internal/adapters/epg
mkdir -p gateway/internal/adapters/providers/opensource
mkdir -p gateway/internal/adapters/providers/xtreamcodes
mkdir -p gateway/internal/api/handlers
mkdir -p gateway/internal/api/middleware

mkdir -p docs/adr
mkdir -p scripts

echo "Estructura de directorios base creada con éxito."
