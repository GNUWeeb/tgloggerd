cmake_minimum_required(VERSION 3.16)

project(tgloggerd
	VERSION 0.1.0
	DESCRIPTION "Telegram logger daemon"
	LANGUAGES C CXX)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
set(CMAKE_C_EXTENSIONS ON)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS ON)

if(NOT CMAKE_BUILD_TYPE)
	set(CMAKE_BUILD_TYPE "Release" CACHE STRING "Build type" FORCE)
endif()

# Build TDLib from the submodule as a shared library.
add_subdirectory(submodules/td EXCLUDE_FROM_ALL)

add_executable(tgloggerd
	src/main.c
	src/tgloggerd/helpers/common.c
	src/tgloggerd/helpers/common.h
	src/tgloggerd/helpers/log.c
	src/tgloggerd/helpers/log.h
	src/tgloggerd/TDLib.cpp
	src/tgloggerd/TDLib.hpp
	src/tgloggerd/tgloggerd_extern.cpp
	src/tgloggerd/tgloggerd_extern.h
	src/tgloggerd/TgLoggerd.cpp
	src/tgloggerd/TgLoggerd.hpp)

target_include_directories(tgloggerd PRIVATE src)
target_compile_definitions(tgloggerd PRIVATE _GNU_SOURCE)
target_compile_options(tgloggerd PRIVATE
	-Wall
	-Wextra)

# Link against the shared TDLib JSON interface. This also propagates the
# TDLib include directories so its interface headers can be included.
target_link_libraries(tgloggerd PRIVATE Td::TdJson)
