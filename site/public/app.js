// Point the download cards at the files of the latest GitHub Release.
// Without JavaScript, or if the API call fails, the cards keep linking to /releases/latest.
(async () => {
  try {
    const res = await fetch('https://api.github.com/repos/jordilopezr/gcetools/releases/latest', {
      headers: { Accept: 'application/vnd.github+json' },
    });
    if (!res.ok) return;
    const rel = await res.json();
    const assets = rel.assets || [];
    document.querySelectorAll('[data-asset]').forEach((card) => {
      const a = assets.find((x) => x.name.endsWith('-' + card.dataset.asset));
      if (a) card.href = a.browser_download_url;
    });
    const date = rel.published_at
      ? new Date(rel.published_at).toLocaleDateString('es', { day: 'numeric', month: 'long', year: 'numeric' })
      : '';
    document.getElementById('release-info').textContent =
      `Versión ${rel.tag_name}${date ? ' · publicada el ' + date : ''}`;
    document.getElementById('version-line').textContent =
      `${rel.tag_name} · Windows · macOS · Linux · Software libre (GPL-3.0)`;
  } catch {
    /* keep the static fallback links */
  }
})();
