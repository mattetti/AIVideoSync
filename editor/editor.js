function showTooltip(x, y, text) {
  const tooltip = document.getElementById('tooltip');
  tooltip.innerText = text;
  tooltip.style.left = `${x}px`;
  tooltip.style.top = `${y + 20}px`; // Position slightly below the cursor
  tooltip.style.display = 'block';
}

function hideTooltip() {
  const tooltip = document.getElementById('tooltip');
  tooltip.style.display = 'none';
}


document.addEventListener('DOMContentLoaded', function() {
  let keyframes = [];
  let videoFileName = ''; // Store the video filename
  let videoDuration = 0;
  let isDragging = false;
  let selectedKeyframeIndex = -1;

  const dropZone = document.getElementById('drop_zone');
  const video = document.getElementById('video');
  const canvas = document.getElementById('keyframeCanvas');
  const ctx = canvas.getContext('2d');
  const addKeyframeButton = document.getElementById('addKeyframe');
  const saveKeyframesButton = document.getElementById('saveKeyframes');
  const clearKeyframesButton = document.getElementById('clearKeyframes');

  function resizeCanvas() {
    // Match canvas width to video's current width
    const width = video.offsetWidth;
    canvas.width = width; // Set canvas width
    canvas.height = 40; // Set canvas height (fixed height for now, but can be dynamic based on video aspect ratio in the future)
    drawKeyframes(); // Redraw keyframes because resizing clears the canvas
  }

  function drawKeyframes() {
    ctx.clearRect(0, 0, canvas.width, canvas.height); // Clear canvas
    keyframes.forEach((keyframe, index) => {
        let x = (keyframe.time / videoDuration) * canvas.width;
        keyframe.x = x; // Store current drawing X for hit detection
        ctx.beginPath();
        ctx.arc(x, canvas.height / 2, 5, 0, Math.PI * 2, true);
        ctx.fill();
    });
}


  window.addEventListener('resize', resizeCanvas); // Adjust canvas size on window resize

  canvas.addEventListener('click', (event) => {
    if (event.button !== 0) return; // Ignore if not left click
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    const clickedKeyframe = keyframes.find(kf => {
        return Math.sqrt((kf.x - x) ** 2 + ((canvas.height / 2) - y) ** 2) <= 5;
    });
    if (clickedKeyframe) {
        video.currentTime = clickedKeyframe.time;
    }
  });

  canvas.addEventListener('mousemove', (event) => {
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    const isOverKeyframe = keyframes.some(kf => {
        return Math.sqrt((kf.x - x) ** 2 + ((canvas.height / 2) - y) ** 2) <= 5;
    });

    canvas.style.cursor = isOverKeyframe ? 'pointer' : 'default';

    if (isOverKeyframe) {
        // Show tooltip
        showTooltip(event.clientX, event.clientY, "Left-click to jump, Right-click to delete");
    } else {
        // Hide tooltip
        hideTooltip();
    }
  });

  canvas.addEventListener('contextmenu', (event) => {
    event.preventDefault(); // Prevent the context menu
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const clickedKeyframeIndex = keyframes.findIndex(kf => {
        return Math.sqrt((kf.x - x) ** 2) <= 5; // Simple hit detection
    });
    if (clickedKeyframeIndex !== -1) {
        keyframes.splice(clickedKeyframeIndex, 1);
        drawKeyframes();
    }
  });

  dropZone.addEventListener('dragover', (event) => {
      event.stopPropagation();
      event.preventDefault();
      event.dataTransfer.dropEffect = 'copy';
  });

  dropZone.addEventListener('drop', (event) => {
    event.stopPropagation();
    event.preventDefault();
    const files = event.dataTransfer.files;
    if (files.length) {
        const file = files[0];
        if (file.type === "application/json") {
            // Handle JSON file
            const reader = new FileReader();
            reader.onload = function(e) {
                try {
                    const json = JSON.parse(e.target.result);
                    // Validate the JSON structure
                    if (Array.isArray(json) && json.every(kf => 'time' in kf)) {
                        keyframes = json; // Load the keyframes
                        drawKeyframes(); // Redraw the canvas with new keyframes
                        console.log("Keyframes loaded from JSON.");
                    } else {
                        console.error("Invalid JSON structure for keyframes.");
                    }
                } catch (error) {
                    console.error("Error parsing JSON:", error);
                }
            };
            reader.readAsText(file);
        } else if (file.type.startsWith('video')) {
            // Handle video file
            video.src = URL.createObjectURL(file);
            video.style.display = 'block';
            videoFileName = file.name.substring(0, file.name.lastIndexOf('.')) || file.name; // Remove extension
            video.onloadedmetadata = () => {
                videoDuration = video.duration;
                resizeCanvas();
            };
            // Update drop zone text
            dropZone.innerHTML = "Drop a new video or a keyframe JSON file here";
        }
    }
  });


  addKeyframeButton.addEventListener('click', () => {
      const currentTime = video.currentTime;
      keyframes.push({time: currentTime});
      drawKeyframes();
      console.log(`Keyframe added at time: ${currentTime}`);
  });

  saveKeyframesButton.addEventListener('click', () => {
      // Create a new array without the 'x' property for each keyframe
      const keyframesToSave = keyframes.map(({x, ...rest}) => rest);
      const blob = new Blob([JSON.stringify(keyframesToSave, null, 2)], {type : 'application/json'});
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${videoFileName}-keyframes.json`; // Use the video filename for the JSON file
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
  });

  clearKeyframesButton.addEventListener('click', () => {
      keyframes = [];
      drawKeyframes();
      console.log("Keyframes cleared.");
  });

});
