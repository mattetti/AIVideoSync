document.addEventListener('DOMContentLoaded', function() {
  let keyframes = [];
  let videoFileName = ''; // Store the video filename

  const dropZone = document.getElementById('drop_zone');
  const video = document.getElementById('video');
  const addKeyframeButton = document.getElementById('addKeyframe');
  const saveKeyframesButton = document.getElementById('saveKeyframes');

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
          video.src = URL.createObjectURL(file);
          video.style.display = 'block';
          videoFileName = file.name.substring(0, file.name.lastIndexOf('.')) || file.name; // Remove extension
      }
  });

  addKeyframeButton.addEventListener('click', () => {
      const currentTime = video.currentTime;
      keyframes.push({time: currentTime});
      console.log(`Keyframe added at time: ${currentTime}`);
  });

  saveKeyframesButton.addEventListener('click', () => {
      const blob = new Blob([JSON.stringify(keyframes, null, 2)], {type : 'application/json'});
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${videoFileName}-keyframes.json`; // Use the video filename for the JSON file
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
  });
});
