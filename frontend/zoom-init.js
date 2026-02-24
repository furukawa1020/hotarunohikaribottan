document.addEventListener("DOMContentLoaded", async () => {
    let roomId = "default";
    let pid = "local-" + Math.floor(Math.random() * 10000);
    let zoomContextStr = "";

    const btn = document.getElementById("vote-btn");
    const statusText = document.querySelector(".status-text");

    try {
        // Zoom Apps SDK Init
        if (typeof zoomSdk !== 'undefined') {
            const configResponse = await zoomSdk.config({
                popoutSize: { width: 480, height: 360 },
                capabilities: ['getMeetingContext', 'getAppContext']
            });
            console.log("Zoom SDK Configured:", configResponse);

            try {
                const appCtx = await zoomSdk.getAppContext();
                if (appCtx && appCtx.context) {
                    zoomContextStr = appCtx.context;
                } else {
                    document.querySelector(".status-text").innerHTML = "<span style='color:red'>エラー: Zoomから認証情報が取得できません</span>";
                }
            } catch (ctxErr) {
                console.warn("Could not get getAppContext", ctxErr);
                document.querySelector(".status-text").innerHTML = "<span style='color:red'>SDKエラー: " + String(ctxErr) + "</span>";
            }
        } else {
            document.querySelector(".status-text").innerHTML = "<span style='color:orange'>Warning: Not running in Zoom Client</span>";
        }
    } catch (e) {
        console.warn("Zoom SDK failed or not running in Zoom Client", e);
        document.querySelector(".status-text").innerHTML = "<span style='color:red'>SDK Init Error: " + String(e) + "</span>";
    }

    // Connect HTMX to WebSocket
    const appContainer = document.getElementById("app");

    // Dynamic wsUrl based on current window location
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;

    const urlParams = new URLSearchParams(window.location.search);
    roomId = urlParams.get('roomId') || "test-room";
    pid = urlParams.get('pid') || pid;

    let wsUrl = `${protocol}//${host}/ws?roomId=${encodeURIComponent(roomId)}&pid=${encodeURIComponent(pid)}`;
    if (zoomContextStr) {
        wsUrl += `&zoom_context=${encodeURIComponent(zoomContextStr)}`;
    }

    appContainer.setAttribute("hx-ext", "ws");
    appContainer.setAttribute("ws-connect", wsUrl);

    // HTMX Initialization Request
    htmx.process(appContainer);

    // Initial UI Setup
    btn.removeAttribute("disabled");
    statusText.innerHTML = "待機中 <span class='anonym-info'>(匿名)</span>";

    // Handle audio autoplay policy workaround
    // Audio context might be restricted. Ensure button click enables audio context.
    btn.addEventListener('click', () => {
        // By clicking the vote button, we satisfy the user interaction requirement for later Audio autoplay
        const audio = new Audio();
        audio.play().catch(() => { }).then(() => { audio.pause(); });
    });
});
