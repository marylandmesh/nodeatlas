$(document).ready(function() {

    addJSFiles();

    $.ajaxSetup({cache:true});

    $('#addme').tooltip();
    $('#distance').tooltip();
    $('#legend').popover({'content':'<img src="/img/node.png" width="16px"/>&nbsp;Active Residential Node<hr><img src="/img/vps.png" width="16px">&nbsp;Active Hosted/Virtual Node<hr><img src="/img/inactive.png" width="16px">&nbsp;Inactive Node', 'html': true});

    // this handles loading the child nodes now
    loadChildMaps();

    $('#search').keyup(function() {
	// TODO display these in some fashion
	var searchResults = search(nodes, $(this).val());
    });

    map.addControl(new L.Control.Scale());

    // If you're at /verify/xxx
    var key = verifying();
    if (key != '') {
	     verifyNode(key);
    }

    if (readonly) {
      addDBWarning();
      $('#addme').remove();
    }

    $(window).bind('hashchange', onHashChange);
    $(window).trigger('hashchange');
    map.on('moveend', function() {
	window.location.hash = getPermalink();
    });
});

$(window).resize(function () {
    $('#map').css('height', ($(window).height()));
    FixNavbar();
}).resize();

function geoLocate() {
    map.locate({setView: true, maxZoom: 17});
    map.on('locationfound', function() {
	map.off('locationfound');
    });
}

function getPermalink() {
    var center = map.getCenter();
    var zoom = map.getZoom();
    return "#" + zoom + "/" + center.lat.toFixed(3) +
	"/" + center.lng.toFixed(3);
}

function loadChildMaps() {
    $.getJSON("/api/child_maps", function(response) {
	for (i in response.data) {
	    mapObj = response.data[i]
	    cachedMaps[mapObj.ID] = {
		"name": mapObj.Name,
		"hostname": mapObj.Hostname
	    }
	}
	addNodes();
	getConnections();
    });
}

function nodexxx(node) {
    var path = window.location.pathname.split('/');
    if (path[1] != "node") return false;
    return (path[2] == node);
}

function verifying() {
    var path = window.location.pathname.split('/');
    if (path[1] != "verify") return '';
    else return path[2];
}

function onMapClick(e) {
    var markerLocation = new L.LatLng(e.latlng.lat, e.latlng.lng);
    var marker = new L.Marker(markerLocation, {icon: newUserIcon});
    newUser.clearLayers();
    newUser.addLayer(marker);
    if ($('#inputform').length > 0) {
	// only update lat/lng
	$('#latitude').val(e.latlng.lat.toFixed(6));
	$('#longitude').val(e.latlng.lng.toFixed(6));
    }
    else $('#wrap').append(getForm(e.latlng.lat.toFixed(6), e.latlng.lng.toFixed(6)));
    $('.node, #delete').remove();
    $('#inputform').fadeIn(500);
    $('#name').focus();
    $.ajax ({
	type: 'GET',
	url: '/api/echo',
	success: function(res) {
	    $('#address').val(res.data);
	}
    });
}


function hide(x) {
  $('#bringnavbarback').remove();
    $('.navbar').fadeOut(500, function() {
      $('#wrap').append('<div id="bringnavbarback">Show</div>');
      $('#bringnavbarback').fadeIn(500, function() {
          $('#bringnavbarback').bind('click', function() {
            $('#bringnavbarback').fadeOut(500, function() {
                $('.navbar').fadeIn(500);
        });
      });
    });
  });
}

function FixNavbar() {
  if ($('.navbar').css('height').replace(/px/g, '') > 52) {
    for (;;) {
      var wrong = $('.navbar').css('height').replace(/px/g, '');
      if (52 >= wrong) return;
      var size = $('.navbar-brand').css('font-size').replace(/px/g, '');
      $('.navbar-brand').css('font-size', (--size) + 'px');
    }
  }
}

function onHashChange(e) {
    var fragment = location.hash.slice(1);

    // Try to parse the fragment as a map view.
    var view = fragment.split('/');
    if (view.length == 3) {
	map.setView(view.slice(1,3), view[0]);
    }
}

// If `.Database.VerifyDisabled` is set to true then we want to add a
// warning at the top
function addDBWarning() {
    var warning = '<div class="alert alert-danger" id="alert-left">Database is in read only mode.</div>';
    $('#wrap').append(warning);
}

function addJSFiles() {
    var html = '<script type="text/javascript" src="/js/distance.js"></script>';
    html += '<script type="text/javascript" src="/js/icon.js"></script>';
    html += '<script type="text/javascript" src="/js/status.js"></script>';
    html += '<script type="text/javascript" src="/js/search.js"></script>';
    html += '<script type="text/javascript" src="/js/captcha.js"></script>';
    html += '<script type="text/javascript" src="/js/node.js"></script>';
    html += '<script type="text/javascript" src="/js/verify.js"></script>';
    html += '<script type="text/javascript" src="/js/form.js"></script>';
    html += '<script type="text/javascript" src="/js/layers.js"></script>';
    html += '<script type="text/javascript" src="/js/peers.js"></script>';
    $('head').append(html);
}
