document.addEventListener("DOMContentLoaded", async () => {
    let roomId = "default";
    let pid = "local-" + Math.floor(Math.random() * 10000);

    const btn = document.getElementById("vote-btn");
    const statusText = document.querySelector(".status-text");

    try {
        // Zoom Apps SDK Init
        if (typeof zoomSdk !== 'undefined') {
            const configResponse = await zoomSdk.config({
                popoutSize: { width: 480, height: 360 },
                capabilities: ['getMeetingContext']
            });
            console.log("Zoom SDK Configured:", configResponse);
            
            // Try to get meeting UUID for Room separation
            try {
                const ctx = await zoomSdk.getMeetingContext();
                if (ctx && ctx.meetingUUID) {
                    roomId = ctx.meetingUUID;
                }
            } catch(ctxErr) {
                console.warn("Could not get getMeetingContext (Guest Mode restrict?)", ctxErr);
            }
        }
    } catch (e) {
        console.warn("Zoom SDK failed or not running in Zoom Client", e);
        const urlParams = new URLSearchParams(window.location.search);
        roomId = urlParams.get('roomId') || "test-room";
        pid = urlParams.get('pid') || pid;
    }

    // Connect HTMX to WebSocket
    const appContainer = document.getElementById("app");
    
    // Dynamic wsUrl based on current window location
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws?roomId=${encodeURIComponent(roomId)}&pid=${encodeURIComponent(pid)}`;
    
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
        audio.play().catch(()=>{}).then(()=>{ audio.pause(); });
    });
});
