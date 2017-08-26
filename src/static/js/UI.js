var UI = {
    ShowLoading: function () {},
    HideLoading: function () {}
};

//加载中提示
UI.ShowLoading = function () {
    $("#loading").modal("show");
}

//隐藏加载提示
UI.HideLoading = function () {
    $("#loading").modal("hide");
}