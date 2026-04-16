interface FileEntry {
    filename: string;
    size: string;
    date: string;
    time: string;
    dl: number;
    dlTotal: number;
}

interface DirApiResponse {
    error?: string;
    path: string;
    subdirectories: string[];
    files: FileEntry[];
    titlePrefix: string;
}

document.addEventListener("DOMContentLoaded", function () {

    function getTBody() {
        return document.querySelector('table#list > tbody')
    }

    function clearChildren(el?: Element) {
        const parent = el || getTBody();
        while (parent?.firstChild) {
            parent.removeChild(parent.lastChild!);
        }
    }

    function createDirRow(dirName: string): HTMLAnchorElement {
        const naCol = '-';
        const tbody = getTBody();
        const tr = document.createElement('tr');
        tbody?.appendChild(tr);
        const tdLink = document.createElement('td');
        const tdSize = document.createElement('td');
        const tdDate = document.createElement('td');
        const tdDl = document.createElement('td');
        const tddlTotal = document.createElement('td');
        [tdLink, tdSize, tdDate, tdDl, tddlTotal].forEach(td => tr.appendChild(td));
        tdLink.classList.add('link', 'directory');
        tdSize.classList.add('na', 'size');
        tdDate.classList.add('na', 'date');
        tdDl.classList.add('na', 'dlcount', 'dlRecent');
        tddlTotal.classList.add('na', 'dlcount', 'dlTotal');
        const anchor = document.createElement('a');
        anchor.appendChild(document.createTextNode(anchor.href = `${dirName}/`));
        anchor.title = dirName;
        tdLink.appendChild(anchor);
        tdSize.appendChild(document.createTextNode(naCol));
        tdDate.appendChild(document.createTextNode(naCol));
        tdDl.appendChild(document.createTextNode(naCol));
        tddlTotal.appendChild(document.createTextNode(naCol));
        return anchor;
    }

    function createFileRow(fileEntry: FileEntry) {
        const tbody = getTBody();
        const tr = document.createElement('tr');
        tbody?.appendChild(tr);
        const tdLink = document.createElement('td');
        const tdSize = document.createElement('td');
        const tdDate = document.createElement('td');
        const tdDl = document.createElement('td');
        const tddlTotal = document.createElement('td');
        const timeSpan = document.createElement('span');
        [tdLink, tdSize, tdDate, tdDl, tddlTotal].forEach(td => tr.appendChild(td));
        tdLink.classList.add('link');
        tdSize.classList.add('size');
        tdDate.classList.add('date');
        tdDl.classList.add('dlcount', 'dlRecent');
        timeSpan.classList.add('time');
        tddlTotal.classList.add('dlcount', 'dlTotal');
        const anchor = document.createElement('a');
        anchor.appendChild(document.createTextNode(anchor.href = `${fileEntry.filename}`));
        anchor.title = fileEntry.filename;
        tdLink.appendChild(anchor);
        tdSize.appendChild(document.createTextNode(fileEntry.size));
        tdDate.appendChild(document.createTextNode(`${fileEntry.date} `));
        tdDate.appendChild(timeSpan);
        timeSpan.appendChild(document.createTextNode(fileEntry.time));
        tdDl.appendChild(document.createTextNode(`${fileEntry.dl}`));
        tddlTotal.appendChild(document.createTextNode(`${fileEntry.dlTotal}`));
    }

    async function navigate(url: URL) {
        const reqPath = `${url.protocol}//${url.host}/.api/dir${url.pathname}`;
        try {
            const resp = await fetch(reqPath, {
                headers: {
                    'X-API-Version': '1'
                }
            });
            if (!resp.ok) {
                throw new Error(`Response status: ${resp.status}`);
            }
            const respBody: DirApiResponse = await resp.json();
            const anchors = respBody.subdirectories.map(createDirRow);
            respBody.files.forEach(createFileRow);
            document.querySelectorAll('span.pathname').forEach(pathContainer => {
                clearChildren(pathContainer);
                pathContainer.appendChild(document.createTextNode(respBody.path));
            });
            document.title = `${respBody.titlePrefix}${respBody.path}`;
            setUpLinks(anchors);
        }
        catch {
            window.location.assign(url.toString());
        }
    }

    function loadUrl(url: URL) {
        window.scrollTo(0, 0);
        clearChildren();
        return navigate(url);
    }

    function clickLink(e: PointerEvent) {
        e.preventDefault();
        const anchor: HTMLAnchorElement = e.target as unknown as HTMLAnchorElement;
        const url = new URL(anchor.href);
        window.history.pushState(anchor.href, "", url.pathname);
        return loadUrl(url);
    }

    function setUpLink(el: Element) {
        const anchor: HTMLAnchorElement = el as unknown as HTMLAnchorElement;
        anchor.addEventListener('click', clickLink);
    }

    function setUpLinks(elements: NodeListOf<Element> | Element[]) {
        elements.forEach(setUpLink);
    }

    window.addEventListener('popstate', event => {
        event.state && loadUrl(new URL(event.state));
    });

    window.history.pushState(window.location.href, "", (new URL(window.location.href)).pathname);
    setUpLinks(document.querySelectorAll(".link.directory > a"));
});
