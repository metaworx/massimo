// SPDX-License-Identifier: AGPL-3.0-or-later
// The channel selector of the documentation pages (packaging/site/site.py):
// the channels from /docs/channels.json, and on a change the same page and
// anchor in the other channel, or its index where the page is not there.
'use strict';

(function () {
  const select = document.getElementById('channel');
  if (!select) {
    return;
  }
  const current = select.dataset.channel;
  const page = select.dataset.page;

  fetch('/docs/channels.json')
    .then((r) => (r.ok ? r.json() : []))
    .then((channels) => {
      if (!Array.isArray(channels) || channels.length === 0) {
        return;
      }
      select.replaceChildren();
      for (const c of channels) {
        const option = document.createElement('option');
        option.value = c.channel;
        option.textContent = c.channel + ' (' + c.version + ')';
        option.selected = c.channel === current;
        select.appendChild(option);
      }
    })
    .catch(() => {});

  select.addEventListener('change', () => {
    const base = '/docs/' + encodeURIComponent(select.value) + '/';
    const target = base + page;
    fetch(target, { method: 'HEAD' })
      .then((r) => {
        window.location.href = r.ok ? target + window.location.hash : base + 'index.html';
      })
      .catch(() => {
        window.location.href = base + 'index.html';
      });
  });
})();
