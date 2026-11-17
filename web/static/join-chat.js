/* SPDX-License-Identifier: GPL-2.0-or-later */
/*
 * tgloggerd web -- /join-chat: preview a chat by invite link or username,
 * then confirm the join. The join is sent with the daemon's normalised form
 * of the previewed ref, so it acts on exactly what the admin looked at.
 * Everything from the server is inserted as text, never as HTML.
 */
(function () {
	"use strict";
	if (typeof jQuery === "undefined")
		return;
	var $ = jQuery;

	var $root = $("#join-chat");
	if (!$root.length)
		return;
	var csrf = $root.attr("data-csrf");

	var $form = $("#jc-form"), $ref = $("#jc-ref"), $msg = $("#jc-msg"),
	    $prev = $("#jc-preview"), $card = $("#jc-card"),
	    $join = $("#jc-join"), $result = $("#jc-result");
	var previewed = null;	/* {ref, title} of the chat on the card */

	var TYPES = {
		basic_group: "Group",
		supergroup: "Supergroup",
		channel: "Channel",
		"private": "User or bot"
	};

	function postForm(url, params) {
		params.csrf = csrf;
		return fetch(url, {
			method: "POST",
			headers: { "Content-Type": "application/x-www-form-urlencoded" },
			body: new URLSearchParams(params).toString()
		}).then(function (r) {
			return r.json().catch(function () {
				return { ok: false, error: "Bad response (HTTP " + r.status + ")." };
			});
		}).catch(function () {
			return { ok: false, error: "Network error." };
		});
	}

	function say($el, cls, text) {
		$el.attr("class", "rt-form-msg" + (cls ? " " + cls : "")).text(text || "");
	}

	function reset() {
		previewed = null;
		$card.prop("hidden", true);
		$join.prop("hidden", true).prop("disabled", false);
		say($result, "", "");
	}

	function fill(res) {
		var c = res.chat || {};
		$("#jc-title").text(c.title || "(untitled)");
		$("#jc-f-ref").text(res.ref + (c.username ? "  (@" + c.username + ")" : ""));
		$("#jc-f-type").text(TYPES[c.type] || c.type || "unknown");
		$("#jc-f-members").text(c.member_count ? String(c.member_count) : "unknown");
		$("#jc-f-member").text(c.is_member ? "already a member"
			: c.banned ? "banned from this chat" : "not a member");
		$("#jc-f-approval").text(c.paid ? "paid subscription link (cannot be joined from here)"
			: c.needs_approval ? "sends a join request an administrator must approve"
			: "immediate");
		$card.prop("hidden", false);
	}

	$ref.on("input", reset);
	$("#jc-cancel").on("click", function () {
		reset();
		say($msg, "", "");
		$ref.trigger("focus");
	});

	$form.on("submit", function (e) {
		e.preventDefault();
		reset();
		say($msg, "", "Looking it up…");
		$prev.prop("disabled", true);
		postForm("/join-chat/preview", { ref: $ref.val() || "" }).then(function (res) {
			$prev.prop("disabled", false);
			if (!res.ok) {
				say($msg, "err", res.error || "Could not look that up.");
				return;
			}
			if (!res.chat) {
				say($msg, "err", res.message);
				return;
			}
			say($msg, "", "");
			fill(res);
			var c = res.chat;
			if (res.outcome !== "ok") {
				say($result, "err", res.message);
			} else if (c.is_member) {
				say($result, "ok", "The account is already a member; there is nothing to do.");
			} else if (c.banned) {
				say($result, "err", "The account is banned from this chat.");
			} else {
				previewed = { ref: res.ref, title: c.title || res.ref };
				$join.prop("hidden", false);
			}
		});
	});

	$join.on("click", function () {
		if (!previewed)
			return;
		if (!window.confirm("Join \"" + previewed.title + "\" as the logging account? " +
				    "The join is visible to the chat's members."))
			return;
		$join.prop("disabled", true);
		say($result, "", "Joining…");
		postForm("/join-chat/join", { ref: previewed.ref }).then(function (res) {
			if (!res.ok) {
				$join.prop("disabled", false);
				say($result, "err", res.error || "The join failed.");
				return;
			}
			var good = res.outcome === "joined" || res.outcome === "already_member" ||
				   res.outcome === "request_sent";
			say($result, good ? "ok" : "err", res.message);
			if (res.chat)
				$("#jc-f-member").text(res.chat.is_member ? "member"
					: res.outcome === "request_sent" ? "join request pending" : "not a member");
			/* Retrying makes sense only after a wait or a failure. */
			if (good)
				previewed = null;
			else
				$join.prop("disabled", false);
		});
	});
})();
