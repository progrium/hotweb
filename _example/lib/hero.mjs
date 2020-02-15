export const HeroSection = {
    view: function (vnode) {
        return m("section.hero.is-info.is-medium.is-bold", vnode.children);
    }
}

export const HeroHead = {
    view: function(vnode) {
        return m("div.hero-head", vnode.children);
    }
}

export const HeroBody = {
    view: function(vnode) {
        return m("div.hero-body", vnode.children);
    }
}

