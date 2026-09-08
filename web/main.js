// Browser glue: load the netso wasm client, discover peers, and wire xterm.js to
// a brokered session with the selected peer.
(function () {
  "use strict";

  var term = new Terminal({ cursorBlink: true, fontFamily: "ui-monospace, Menlo, monospace", fontSize: 13, theme: { background: "#0d1117" } });
  term.open(document.getElementById("terminal"));
  term.writeln("netso browser access — pick a network, List peers, choose one, Connect.");

  var statusEl = document.getElementById("status");
  var networkInput = document.getElementById("network");
  var peerSelect = document.getElementById("peer");
  var refreshBtn = document.getElementById("refresh");
  var connectBtn = document.getElementById("connect");

  // Prefill from ?network=...&peer=...
  var params = new URLSearchParams(location.search);
  if (params.get("network")) networkInput.value = params.get("network");

  function setStatus(s) {
    statusEl.textContent = s;
    statusEl.className = /fail|error|denied/i.test(s) ? "err" : /connected/i.test(s) ? "ok" : "";
  }

  var peerKeys = {}; // name -> pubkey
  var session = null;

  async function listPeers() {
    var net = networkInput.value || "prod";
    setStatus("listing peers");
    try {
      var resp = await fetch("peers?network=" + encodeURIComponent(net));
      var peers = await resp.json();
      peerSelect.innerHTML = "";
      peerKeys = {};
      (peers || []).forEach(function (p) {
        peerKeys[p.name] = p.pubkey;
        var opt = document.createElement("option");
        opt.value = p.name; opt.textContent = p.name;
        peerSelect.appendChild(opt);
      });
      var want = params.get("peer");
      if (want && peerKeys[want]) peerSelect.value = want;
      setStatus((peers && peers.length ? peers.length : 0) + " peer(s) on " + net);
    } catch (e) {
      setStatus("error listing peers");
    }
  }

  function connect() {
    if (session) { session.close(); session = null; }
    var net = networkInput.value || "prod";
    var peer = peerSelect.value;
    if (!peer || !peerKeys[peer]) { setStatus("no peer selected (List peers first)"); return; }
    var scheme = location.protocol === "https:" ? "wss://" : "ws://";
    var wsURL = scheme + location.host + "/connect?network=" + encodeURIComponent(net) + "&peer=" + encodeURIComponent(peer);
    term.writeln("\r\n[connecting to " + net + "/" + peer + "]");
    session = window.netsoConnect({
      wsURL: wsURL,
      agentPub: peerKeys[peer],
      onData: function (u8) { term.write(u8); },
      onStatus: function (s) { setStatus(s); },
      onClose: function () { setStatus("disconnected"); session = null; }
    });
    term.focus();
  }

  term.onData(function (d) { if (session) session.send(d); });
  refreshBtn.addEventListener("click", listPeers);
  connectBtn.addEventListener("click", connect);

  (async function () {
    try {
      var go = new Go();
      var bytes = await (await fetch("netso.wasm")).arrayBuffer();
      var result = await WebAssembly.instantiate(bytes, go.importObject);
      go.run(result.instance); // sets window.netsoConnect, then parks
      if (typeof window.netsoConnect !== "function") throw new Error("wasm did not export netsoConnect");
      connectBtn.disabled = false;
      connectBtn.textContent = "Connect";
      setStatus("ready");
      listPeers();
    } catch (e) {
      setStatus("load failed");
      term.writeln("\r\n[wasm load error] " + e);
    }
  })();
})();
