export const HeroSection = {
    view: function (vnode) {
        return <section class="hero is-info is-medium is-bold">{vnode.children}</section>;
    }
}

export const HeroHead = {
    view: function(vnode) {
        return <div class="hero-head">{vnode.children}</div>;
    }
}

export const HeroBody = {
    view: function(vnode) {
        return <div class="hero-body">{vnode.children}</div>;
    }
}

