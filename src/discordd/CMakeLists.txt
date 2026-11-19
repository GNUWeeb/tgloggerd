# SPDX-License-Identifier: GPL-2.0-or-later
#
# discordd -- the Discord daemon: logs Discord messages and forwards them to
# Telegram.
#
# Added by the root CMakeLists AFTER the TDLib and MySQL Connector/C++
# subprojects, because it links Td::TdStatic and connector-jdbc from them.
#
# gwdiscord is pulled in from its own directory as a self-contained project.
# It is intentionally a separate build with no tgloggerd dependency, so it can
# be lifted into its own repository later -- see src/gwdiscord/README.md.
add_subdirectory(${CMAKE_CURRENT_SOURCE_DIR}/../gwdiscord gwdiscord)

add_executable(discordd
	main.cpp
	DiscordD.cpp
	DiscordD.hpp
	DiscordBots.cpp
	DiscordBots.hpp
	DB.cpp
	DB.hpp
	RouteFilter.cpp
	RouteFilter.hpp
	ForwardQueue.cpp
	ForwardQueue.hpp
	TelegramSender.cpp
	TelegramSender.hpp
	# The generic MySQL layer is shared with tgloggerd. It is compiled in
	# here rather than linked from the tgloggerd target, which is an
	# executable; the two files are small and carry no daemon specifics.
	${CMAKE_SOURCE_DIR}/src/mysql/ConnectionPool.cpp
	${CMAKE_SOURCE_DIR}/src/mysql/Database.cpp)

# Land the binary in build/ alongside tgloggerd rather than in
# build/src/discordd/, so run.sh and the compose service use one obvious path.
set_target_properties(discordd PROPERTIES
	RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR})

target_include_directories(discordd PRIVATE ${CMAKE_SOURCE_DIR}/src)
target_compile_definitions(discordd PRIVATE _GNU_SOURCE)
target_compile_options(discordd PRIVATE -Wall -Wextra)

# gwdiscord exposes only its own headers: Boost and jsoncpp are PRIVATE to it,
# so nothing here compiles against either.
target_link_libraries(discordd PRIVATE
	gwdiscord
	connector-jdbc
	Td::TdStatic
	Threads::Threads)

# The Discord bot runner over gwdiscord's mock transport: no network, no
# database, no TDLib. Not part of the default build;
#   cmake --build build --target discordd_bots_test && build/discordd_bots_test
add_executable(discordd_bots_test EXCLUDE_FROM_ALL
	tests/discord_bots_test.cpp
	DiscordBots.cpp
	DiscordBots.hpp)
set_target_properties(discordd_bots_test PROPERTIES
	RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR})
target_compile_options(discordd_bots_test PRIVATE -Wall -Wextra)
target_link_libraries(discordd_bots_test PRIVATE gwdiscord Threads::Threads)

# The per-bot worker queues and the per-chat send pacing; no network, no
# database. Not part of the default build;
#   cmake --build build --target discordd_forward_queue_test &&
#   build/discordd_forward_queue_test
add_executable(discordd_forward_queue_test EXCLUDE_FROM_ALL
	tests/forward_queue_test.cpp
	ForwardQueue.cpp
	ForwardQueue.hpp)
set_target_properties(discordd_forward_queue_test PROPERTIES
	RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR})
target_compile_options(discordd_forward_queue_test PRIVATE -Wall -Wextra)
target_link_libraries(discordd_forward_queue_test PRIVATE Threads::Threads)

# Which forwarding flag each Discord attachment follows; no network, no
# database. Not part of the default build;
#   cmake --build build --target discordd_route_filter_test &&
#   build/discordd_route_filter_test
add_executable(discordd_route_filter_test EXCLUDE_FROM_ALL
	tests/route_filter_test.cpp
	RouteFilter.cpp
	RouteFilter.hpp)
set_target_properties(discordd_route_filter_test PROPERTIES
	RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR})
target_compile_options(discordd_route_filter_test PRIVATE -Wall -Wextra)
target_link_libraries(discordd_route_filter_test PRIVATE gwdiscord)
